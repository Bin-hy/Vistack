package discovery

import (
	"context"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

// EtcdBuilder 基于 etcd 前缀的 gRPC resolver.Builder（scheme="etcd"）
type EtcdBuilder struct {
	client *clientv3.Client
	prefix string
}

func NewEtcdBuilder(client *clientv3.Client, prefix string) *EtcdBuilder {
	return &EtcdBuilder{client: client, prefix: prefix}
}

func (b *EtcdBuilder) Scheme() string { return "etcd" }

func (b *EtcdBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &etcdResolver{
		client: b.client,
		prefix: b.prefix,
		cc:     cc,
		done:   make(chan struct{}),
	}
	r.update()
	go r.watch()
	return r, nil
}

type etcdResolver struct {
	client *clientv3.Client
	prefix string
	cc     resolver.ClientConn
	done   chan struct{}
	once   sync.Once
}

func (r *etcdResolver) watch() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := r.client.Watch(ctx, r.prefix, clientv3.WithPrefix())
	for {
		select {
		case <-r.done:
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			r.update()
		}
	}
}

func (r *etcdResolver) update() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := r.client.Get(ctx, r.prefix, clientv3.WithPrefix())
	if err != nil {
		return
	}
	addresses := make([]resolver.Address, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		addr := string(kv.Value)
		if addr != "" {
			addresses = append(addresses, resolver.Address{Addr: addr})
		}
	}
	r.cc.UpdateState(resolver.State{Addresses: addresses})
}

func (r *etcdResolver) ResolveNow(o resolver.ResolveNowOptions) { r.update() }

func (r *etcdResolver) Close() {
	r.once.Do(func() { close(r.done) })
}
