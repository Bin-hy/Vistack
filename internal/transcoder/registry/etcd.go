package registry

import (
	"context"
	"fmt"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	leaseTTL          = 10
	keepAliveInterval = 3 * time.Second
)

// Registrar 在 etcd 中注册一个服务实例并通过租约保活
type Registrar struct {
	client *clientv3.Client
	lease  clientv3.LeaseID
	key    string
	addr   string
	stop   chan struct{}
}

// Register 以 {prefix}/{id} 为 key、addr 为 value 注册，租约 TTL 10s、每 3s 保活
func Register(ctx context.Context, client *clientv3.Client, prefix, id, addr string) (*Registrar, error) {
	lease, err := client.Grant(ctx, leaseTTL)
	if err != nil {
		return nil, fmt.Errorf("grant lease failed: %w", err)
	}
	key := fmt.Sprintf("%s/%s", prefix, id)
	if _, err := client.Put(ctx, key, addr, clientv3.WithLease(lease.ID)); err != nil {
		_, _ = client.Revoke(ctx, lease.ID)
		return nil, fmt.Errorf("register failed: %w", err)
	}

	r := &Registrar{client: client, lease: lease.ID, key: key, addr: addr, stop: make(chan struct{})}
	go r.keepAlive()
	return r, nil
}

// keepAlive 周期续约；失败时重新 grant + put
func (r *Registrar) keepAlive() {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if _, err := r.client.KeepAliveOnce(ctx, r.lease); err != nil {
				if lease, gerr := r.client.Grant(ctx, leaseTTL); gerr == nil {
					r.lease = lease.ID
					_, _ = r.client.Put(ctx, r.key, r.addr, clientv3.WithLease(lease.ID))
				}
			}
			cancel()
		}
	}
}

// Close 撤销租约并停止保活
func (r *Registrar) Close() error {
	close(r.stop)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := r.client.Revoke(ctx, r.lease)
	return err
}
