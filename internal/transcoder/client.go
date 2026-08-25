package transcoder

import (
	"context"
	"fmt"
	"time"

	"github.com/binhy/vistack/internal/config"
	"github.com/binhy/vistack/internal/discovery"
	transcoderpb "github.com/binhy/vistack/internal/transcoder/pb/transcoder/v1"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client 封装 transcoder gRPC 客户端（etcd 发现或静态地址）
type Client struct {
	pb   transcoderpb.TranscoderServiceClient
	conn *grpc.ClientConn
	etcd *clientv3.Client
}

// NewClient 根据配置构造客户端；UseEtcd 为 true 且 etcd 端点非空时走 etcd 发现，否则静态地址
func NewClient(ctx context.Context, cfg *config.AppConfig) (*Client, error) {
	if cfg.Transcoder.UseEtcd && len(cfg.Etcd.Endpoints) > 0 {
		cli, err := clientv3.New(clientv3.Config{Endpoints: cfg.Etcd.Endpoints, DialTimeout: 5 * time.Second})
		if err != nil {
			return nil, fmt.Errorf("connect etcd failed: %w", err)
		}
		prefix := cfg.Etcd.Prefix
		if prefix == "" {
			prefix = defaultTranscoderPrefix
		}
		conn, err := grpc.NewClient(
			"etcd:///"+prefix,
			grpc.WithResolvers(discovery.NewEtcdBuilder(cli, prefix)),
			grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			_ = cli.Close()
			return nil, fmt.Errorf("dial transcoder (etcd) failed: %w", err)
		}
		return &Client{pb: transcoderpb.NewTranscoderServiceClient(conn), conn: conn, etcd: cli}, nil
	}

	if cfg.Transcoder.Addr == "" {
		return nil, fmt.Errorf("transcoder addr is empty and etcd discovery is disabled")
	}
	conn, err := grpc.NewClient(cfg.Transcoder.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial transcoder failed: %w", err)
	}
	return &Client{pb: transcoderpb.NewTranscoderServiceClient(conn), conn: conn}, nil
}

func (c *Client) ProcessVideo(ctx context.Context, req *transcoderpb.ProcessVideoRequest) (*transcoderpb.ProcessVideoResponse, error) {
	return c.pb.ProcessVideo(ctx, req)
}

func (c *Client) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	if c.etcd != nil {
		_ = c.etcd.Close()
	}
}
