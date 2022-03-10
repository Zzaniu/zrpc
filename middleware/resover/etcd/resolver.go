/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/4 17:36
Desc   :

    ......................我佛慈悲......................

                           _oo0oo_
                          o8888888o
                          88" . "88
                          (| -_- |)
                          0\  =  /0
                        ___/`---'\___
                      .' \\|     |// '.
                     / \\|||  :  |||// \
                    / _||||| -卍-|||||- \
                   |   | \\\  -  /// |   |
                   | \_|  ''\---/''  |_/ |
                   \  .-\__  '-'  ___/-. /
                 ___'. .'  /--.--\  `. .'___
              ."" '<  `.___\_<|>_/___.' >' "".
             | | :  `- \`.;`\ _ /`;.`/ - ` : | |
             \  \ `_.   \_ __\ /__ _/   .-` /  /
         =====`-.____`.___ \_____/___.-`___.-'=====
                           `=---='

    ..................佛祖保佑, 永无BUG...................

*/

package etcd

import (
	"context"
	"errors"
	"google.golang.org/grpc/resolver"
	"strings"
	"time"
	"zrpc/middleware/register"
	strSet "zrpc/tool/set/str-set"
	"zrpc/tool/xlog"
)

const (
	Name = "discovery"
)

type (
	ResolverBuilderEtcd struct {
		discover register.IDiscovery
		timeout  time.Duration
	}

	discoveryResolver struct {
		w      register.IWatcher
		cc     resolver.ClientConn
		ctx    context.Context
		cancel context.CancelFunc
	}
)

func NewResolverBuilderEtcd(discoverer register.IDiscovery) resolver.Builder {
	return &ResolverBuilderEtcd{discover: discoverer, timeout: time.Second * 5}
}

// Build 做一些解析，解析ETCD的地址，从ETCD解析到RPC的地址，用cc.UpdateState更新RPC地址, 执行 grpc.dial 的时候调用
func (b *ResolverBuilderEtcd) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	var (
		err error
		w   register.IWatcher
	)
	done := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// 得到服务名，然后生成一个Watcher来监听服务
		w, err = b.discover.Watch(ctx, strings.TrimPrefix(target.URL.Path, "/"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(b.timeout):
		err = errors.New("discovery create watcher overtime")
	}
	if err != nil {
		cancel()
		return nil, err
	}
	r := &discoveryResolver{
		w:      w, // Watcher
		cc:     cc,
		ctx:    ctx,
		cancel: cancel,
	}
	go r.watch()
	return r, nil
}

// Scheme 返回服务发现名
func (*ResolverBuilderEtcd) Scheme() string {
	return Name
}

// watch 监控服务并更新服务
func (d *discoveryResolver) watch() {
	for {
		select {
		case <-d.ctx.Done():
			return
		default:
		}
		// d.w.Next()会阻塞，直到有服务上线或者下线的话，或者被取消
		ins, err := d.w.Next()
		if err != nil {
			// 如果是取消了, 那么直接返回
			if errors.Is(err, context.Canceled) {
				return
			}
			xlog.XLog.Errorf("[resolver] 获取服务失败, err: %v", err)
			time.Sleep(time.Second)
			continue
		}
		d.update(ins)
	}
}

// update 在服务上线或者下线的时候更新服务地址, 是一次性更新所有
func (d *discoveryResolver) update(ins []*register.ServiceInstance) {
	addrs := make([]resolver.Address, 0)
	endpoints := strSet.NewStrSet()
	for _, in := range ins {
		// in: {"name":"Dev/user.rpc","key":"Dev/user.rpc/1646557037/b2a0adad-8c66-cdec-aab3-bc7dc27b26c9","endpoint":"127.0.0.1:8083"}
		// TODO 这里是不是可以判断一下从ETCD拿到的数据是否OK
		if len(in.Endpoint) == 0 || endpoints.Contains(in.Endpoint) {
			continue
		}
		endpoints.Add(in.Endpoint)
		// TODO Attributes 这个好像是传一些MD之类的信息吧，token? 不是很清楚
		addr := resolver.Address{
			ServerName: in.Name,
			Addr:       in.Endpoint,
		}
		addr.Attributes = addr.Attributes.WithValue("rawServiceInstance", in)
		addrs = append(addrs, addr)
	}
	if len(addrs) == 0 {
		xlog.XLog.Warn("[resolver] 未找到服务")
		return
	}
	// 更新地址信息
	err := d.cc.UpdateState(resolver.State{Addresses: addrs})
	if err != nil {
		xlog.XLog.Errorf("[resolver] 更新服务失败, err: %s", err)
	}
}

// ResolveNow 从etcd解析并更新rpc地址
func (d discoveryResolver) ResolveNow(options resolver.ResolveNowOptions) {}

func (d discoveryResolver) Close() {
	d.cancel()
	err := d.w.Stop()
	if err != nil {
		xlog.XLog.Errorf("[resolver] 停止watch失败, err: %s", err)
	}
}
