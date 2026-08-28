package authclient

import (
	"context"
	"fmt"
	"time"

	authpb "github.com/binhy/vistack/internal/auth/pb/auth/v1"
	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/discovery"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserClient 封装 auth 服务用户查询 gRPC 客户端（etcd 发现或静态地址）。
type UserClient struct {
	pb   authpb.UserServiceClient
	conn *grpc.ClientConn
	etcd *clientv3.Client
}

// NewUserClient 构造客户端：etcd 端点非空时走 etcd 发现，否则用静态 grpc_addr。
func NewUserClient(ctx context.Context, cfg *config.AppConfig) (*UserClient, error) {
	if len(cfg.Etcd.Endpoints) > 0 {
		cli, err := clientv3.New(clientv3.Config{Endpoints: cfg.Etcd.Endpoints, DialTimeout: 5 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("connect etcd failed: %w", err)
		}
		prefix := config.DefaultAuthPrefix
		conn, err := grpc.NewClient(
			"etcd:///"+prefix,
			grpc.WithResolvers(discovery.NewEtcdBuilder(cli, prefix)),
			grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			_ = cli.Close()
			return nil, fmt.Errorf("dial auth (etcd) failed: %w", err)
		}
		return &UserClient{pb: authpb.NewUserServiceClient(conn), conn: conn, etcd: cli}, nil
	}

	addr := cfg.AuthService.GRPCAddr
	if addr == "" {
		addr = "localhost:50052"
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial auth failed: %w", err)
	}
	return &UserClient{pb: authpb.NewUserServiceClient(conn), conn: conn}, nil
}

// GetUserInfos 批量查询用户公开信息，返回 user_id -> UserInfo 映射。
func (c *UserClient) GetUserInfos(ctx context.Context, ids []int64) (map[int64]*authpb.UserInfo, error) {
	resp, err := c.pb.GetUserInfos(ctx, &authpb.GetUserInfosRequest{UserIds: ids})
	if err != nil {
		return nil, err
	}
	m := make(map[int64]*authpb.UserInfo, len(resp.GetUsers()))
	for _, u := range resp.GetUsers() {
		m[u.GetId()] = u
	}
	return m, nil
}

func (c *UserClient) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	if c.etcd != nil {
		_ = c.etcd.Close()
	}
}
