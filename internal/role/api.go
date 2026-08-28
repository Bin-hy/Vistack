package role

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	v1 "github.com/binhy/vistack/internal/api/v1"
	"github.com/binhy/vistack/internal/authclient"
	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/core"
	"github.com/binhy/vistack/internal/routers"
	authpkg "github.com/binhy/vistack/pkg/auth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// apiShutdownTimeout 优雅停机等待在途请求的上限
const apiShutdownTimeout = 30 * time.Second

// RunAPI 启动 api 角色：HTTP 服务 + 上传/预签名 + 投递 Kafka 消息。
// 认证与用户资料已迁至 auth 服务；本角色不持私钥，通过 JWKS 本地验签。
func RunAPI(cfg *config.AppConfig) {
	core.InitDB(cfg)
	core.InitMinioClient(cfg)
	core.InitRedis(cfg)
	core.InitSnowflake(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// JWKS 本地验签器（不持私钥、不回调 auth）
	jwksURL := cfg.AuthService.JWKSURL
	if jwksURL == "" {
		jwksURL = defaultJWKSURL(cfg)
	}
	verifier := authpkg.NewTokenVerifier(jwksURL)
	verifier.StartAutoRefresh(ctx, time.Hour)

	// auth 用户查询客户端（作者展示等场景批量查询）
	userClient, err := authclient.NewUserClient(ctx, cfg)
	if err != nil {
		if core.Logger != nil {
			core.Logger.Error("create auth user client failed", zap.Error(err))
		}
		panic(err)
	}
	defer userClient.Close()
	v1.SetUserClient(userClient)

	core.InitKafka(cfg)
	defer core.CloseKafka()

	switch cfg.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	r := core.NewServer()
	routers.RegisterRoutes(r, verifier)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			if core.Logger != nil {
				core.Logger.Error("http server failed", zap.Error(err))
			}
			panic(err)
		}
	case <-ctx.Done():
		if core.Logger != nil {
			core.Logger.Info("api shutting down: draining in-flight requests")
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), apiShutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			if core.Logger != nil {
				core.Logger.Error("graceful shutdown failed, force exit", zap.Error(err))
			}
			os.Exit(1)
		}
		if core.Logger != nil {
			core.Logger.Info("api exited cleanly")
		}
	}
}

// defaultJWKSURL 从 auth_service.http_addr 与 auth.jwks_path 构造本机 JWKS URL（兜底）。
func defaultJWKSURL(cfg *config.AppConfig) string {
	addr := cfg.AuthService.HTTPAddr
	if addr == "" {
		addr = ":8081"
	}
	port := strings.TrimPrefix(addr, ":")
	if i := strings.LastIndex(addr, ":"); i >= 0 {
		port = addr[i+1:]
	}
	path := cfg.Auth.JWKSPath
	if path == "" {
		path = "/.well-known/jwks.json"
	}
	return "http://127.0.0.1:" + port + path
}
