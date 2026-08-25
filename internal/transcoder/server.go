package transcoder

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	transcoderpb "github.com/binhy/vistack/internal/transcoder/pb/transcoder/v1"
	"github.com/binhy/vistack/internal/transcoder/registry"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const defaultTranscoderPrefix = "/vistack/transcoders"

// RunServer 启动 transcoder 的 gRPC 服务并向 etcd 注册
func RunServer(ctx context.Context, cfg *config.AppConfig) error {
	lis, err := net.Listen("tcp", cfg.Transcoder.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s failed: %w", cfg.Transcoder.ListenAddr, err)
	}

	gs := grpc.NewServer()
	transcoderpb.RegisterTranscoderServiceServer(gs, NewService())

	var registrar *registry.Registrar
	var etcdCli *clientv3.Client
	if len(cfg.Etcd.Endpoints) > 0 {
		etcdCli, err = clientv3.New(clientv3.Config{Endpoints: cfg.Etcd.Endpoints, DialTimeout: 5 * time.Second})
		if err != nil {
			_ = lis.Close()
			return fmt.Errorf("connect etcd failed: %w", err)
		}
		defer etcdCli.Close()

		prefix := cfg.Etcd.Prefix
		if prefix == "" {
			prefix = defaultTranscoderPrefix
		}
		addr := advertiseAddr(cfg.Transcoder.ListenAddr)
		registrar, err = registry.Register(ctx, etcdCli, prefix, uuid.New().String(), addr)
		if err != nil {
			_ = lis.Close()
			return fmt.Errorf("register to etcd failed: %w", err)
		}
		defer registrar.Close()

		if core.Logger != nil {
			core.Logger.Info("transcoder registered to etcd", zap.String("prefix", prefix), zap.String("addr", addr))
		}
	}

	if core.Logger != nil {
		core.Logger.Info("transcoder gRPC server listening", zap.String("addr", cfg.Transcoder.ListenAddr))
	}

	errCh := make(chan error, 1)
	go func() { errCh <- gs.Serve(lis) }()

	select {
	case <-ctx.Done():
		gs.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}

// advertiseAddr 计算对外可访问的注册地址：优先 POD_IP（k8s），其次本机 IP，最后 hostname
func advertiseAddr(listenAddr string) string {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		port = ""
	}
	if port == "" {
		port = "50051"
	}

	host := os.Getenv("POD_IP")
	if host == "" {
		host = localIP()
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

// localIP 返回第一个非回环 IPv4 地址
func localIP() string {
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
