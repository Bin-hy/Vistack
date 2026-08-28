package role

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/binhy/vistack/internal/auth"
	authpb "github.com/binhy/vistack/internal/auth/pb/auth/v1"
	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/transcoder/registry"
	authpkg "github.com/binhy/vistack/pkg/auth"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const authShutdownTimeout = 30 * time.Second

// RunAuth 启动 auth 角色：HTTP（注册/登录/资料/JWKS）+ gRPC（用户查询）+ etcd 注册。
// 持有 RSA 私钥签发 RS256 JWT；私钥经环境变量注入，缺失时生成临时密钥（仅开发）。
func RunAuth(cfg *config.AppConfig) {
	core.InitDB(cfg)
	core.InitMinioClient(cfg)

	privateKeyPEM, err := loadOrGeneratePrivateKey()
	if err != nil {
		panic(fmt.Sprintf("load private key failed: %v", err))
	}

	tm, err := authpkg.NewTokenManager(privateKeyPEM, cfg.Auth.Kid, cfg.Auth.Issuer, time.Duration(cfg.Auth.JWTExpiration)*time.Second)
	if err != nil {
		panic(fmt.Sprintf("init token manager failed: %v", err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// gRPC server + etcd 注册（供 api 经 etcd 发现）
	grpcLis, err := net.Listen("tcp", cfg.AuthService.GRPCAddr)
	if err != nil {
		panic(fmt.Sprintf("listen gRPC %s failed: %v", cfg.AuthService.GRPCAddr, err))
	}
	gs := grpc.NewServer()
	authpb.RegisterUserServiceServer(gs, auth.NewService())

	var registrar *registry.Registrar
	var etcdCli *clientv3.Client
	if len(cfg.Etcd.Endpoints) > 0 {
		etcdCli, err = clientv3.New(clientv3.Config{Endpoints: cfg.Etcd.Endpoints, DialTimeout: 5 * time.Second})
		if err != nil {
			_ = grpcLis.Close()
			panic(fmt.Sprintf("connect etcd failed: %v", err))
		}
		defer etcdCli.Close()

		registrar, err = registry.Register(ctx, etcdCli, config.DefaultAuthPrefix, uuid.New().String(), advertiseAuthAddr(cfg.AuthService.GRPCAddr))
		if err != nil {
			_ = grpcLis.Close()
			panic(fmt.Sprintf("register auth to etcd failed: %v", err))
		}
		defer registrar.Close()
		if core.Logger != nil {
			core.Logger.Info("auth registered to etcd", zap.String("prefix", config.DefaultAuthPrefix), zap.String("addr", advertiseAuthAddr(cfg.AuthService.GRPCAddr)))
		}
	}

	go func() { _ = gs.Serve(grpcLis) }()

	// HTTP server（对外认证 + JWKS + 受保护用户路由）
	handler := auth.NewHandler(tm)
	r := core.NewServer()
	auth.RegisterRoutes(r, handler, tm) // tm 作为 TokenValidator（私钥验签，auth 内部不自引用 JWKS）

	srv := &http.Server{Addr: cfg.AuthService.HTTPAddr, Handler: r}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	if core.Logger != nil {
		core.Logger.Info("auth server listening",
			zap.String("http", cfg.AuthService.HTTPAddr),
			zap.String("grpc", cfg.AuthService.GRPCAddr),
		)
	}

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(fmt.Sprintf("auth http server failed: %v", err))
		}
	case <-ctx.Done():
		if core.Logger != nil {
			core.Logger.Info("auth shutting down: draining in-flight requests")
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), authShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			core.Logger.Error("auth graceful shutdown failed, force exit", zap.Error(err))
			os.Exit(1)
		}
		gs.GracefulStop()
		if core.Logger != nil {
			core.Logger.Info("auth exited cleanly")
		}
	}
}

// loadOrGeneratePrivateKey 读取 RSA 私钥：优先环境变量 PEM，其次文件路径，最后生成临时密钥（开发）。
func loadOrGeneratePrivateKey() ([]byte, error) {
	if s := os.Getenv("VISTACK_AUTH_RSA_PRIVATE_KEY"); s != "" {
		return []byte(strings.ReplaceAll(s, "\\n", "\n")), nil
	}
	if p := os.Getenv("VISTACK_AUTH_RSA_PRIVATE_KEY_FILE"); p != "" {
		return os.ReadFile(p)
	}
	// 开发模式：生成临时密钥，进程内有效
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	if core.Logger != nil {
		core.Logger.Warn("no RSA private key configured (VISTACK_AUTH_RSA_PRIVATE_KEY), generated ephemeral key — dev only")
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// advertiseAuthAddr 计算 auth 注册到 etcd 的对外地址：POD_IP 优先，其次本机 IP，最后 hostname。
func advertiseAuthAddr(listenAddr string) string {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		port = "50052"
	}
	host := os.Getenv("POD_IP")
	if host == "" {
		host = localAuthIP()
	}
	if host == "" {
		if h, err := os.Hostname(); err == nil {
			host = h
		}
	}
	if host == "" {
		host = "localhost"
	}
	return net.JoinHostPort(host, port)
}

// localAuthIP 返回第一个非回环 IPv4 地址
func localAuthIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}
