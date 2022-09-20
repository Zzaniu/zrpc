/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/2/28 14:17
Desc   : 这个没用了的, 是最开始用来测试的

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

package resolver

import (
    "context"
    "errors"
    "fmt"
    "github.com/Zzaniu/zrpc/middleware/register"
    etcdClientV3 "go.etcd.io/etcd/client/v3"
    "golang.org/x/xerrors"
    "google.golang.org/grpc/resolver"
    "log"
    "strings"
    "sync"
    "time"
)

// Deprecated
type etcdResolverBuilder struct{}

// Resolver 解析器
// Deprecated
type Resolver struct {
    client    *etcdClientV3.Client
    key       string
    endpoints []string
    cc        resolver.ClientConn
    sync.Mutex
}

// // 注册全局解析器
// func init() {
//     resolver.Register(&etcdResolverBuilder{})
// }

// 获取 rpc 服务的名字
func getRpcKey(target resolver.Target) (string, []string) {
    fmt.Printf("resolver.Target = %#v\n", target)
    var key string
    var endpoint []string
    t := strings.SplitN(target.Endpoint, "/", 2)
    if len(t) == 2 {
        key = t[1] + "/"
    } else {
        fmt.Println("t = ", t)
        panic(errors.New("target 无 rpc key"))
    }
    endpoint = strings.Split(t[0], ",")
    if len(endpoint) < 1 {
        panic(errors.New("target 无 etcd host"))
    }
    return key, endpoint
}

// Build 做一些解析，解析ETCD的地址，从ETCD解析到RPC的地址，用cc.UpdateState更新RPC地址, 执行 grpc.dial 的时候调用
func (s *etcdResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
    fmt.Println("target.Endpoint = ", target.Endpoint)
    key, endpoint := getRpcKey(target)
    client, err := etcdClientV3.New(etcdClientV3.Config{
        Endpoints:   []string{"172.18.2.249:20000", "172.18.2.249:20002", "172.18.2.249:20004"},
        DialTimeout: 1 * time.Second,
    })
    if err != nil {
        log.Fatalf("%+v\n", xerrors.Errorf("链接etcd出错: %w", err))
    }
    r := &Resolver{
        client:    client,
        key:       key,
        endpoints: endpoint,
        cc:        cc,
    }
    r.ResolveNow(resolver.ResolveNowOptions{})

    go r.watch()

    return r, nil
}

func (s *etcdResolverBuilder) Scheme() string {
    return "etcd"
}

// ResolveNow 从etcd解析并更新rpc地址
func (s *Resolver) ResolveNow(opts resolver.ResolveNowOptions) {
    fmt.Println("有变动哦")
    s.doResolve(opts)
}

func (s *Resolver) Close() {
    _ = s.client.Close()
}

// doResolve 解析并更新rpc地址
func (s *Resolver) doResolve(opts resolver.ResolveNowOptions) {
    fmt.Println("doResolve")
    addrs := make([]resolver.Address, len(s.endpoints))
    fmt.Println("endpoints = ", s.endpoints)

    kv := etcdClientV3.NewKV(s.client)
    fmt.Println("s.key = ", s.key)
    resp, err := kv.Get(context.TODO(), s.key, etcdClientV3.WithPrefix())
    if err != nil {
        panic(err)
    }

    log.Printf("Kvs = %v\n", resp.Kvs)
    for i, s := range resp.Kvs {
        fmt.Println("s.Value = ", string(s.Value))
        serviceInstance, err := register.Unmarshal(s.Value)
        if err != nil {
            fmt.Println("err = ", err)
            continue
        }
        fmt.Printf("serviceInstance = %#v\n", serviceInstance.Endpoint)
        addrs[i] = resolver.Address{Addr: serviceInstance.Endpoint}
    }
    // 更新真实RPC地址
    s.Lock()
    err = s.cc.UpdateState(resolver.State{Addresses: addrs})
    s.Unlock()
    if err != nil {
        panic(err)
    }
}

// watch 监控etcd。如果有服务上线或下线，会调用doResolve去更新
func (s *Resolver) watch() {
    fmt.Println("watch")
    watch := s.client.Watch(context.TODO(), s.key, etcdClientV3.WithPrefix())
    fmt.Printf("watch = %v, type = %T\n", watch, watch)
    for wresp := range watch {
        for _, v := range wresp.Events {
            log.Printf("Type: %s Key:%s Value:%s\n", v.Type, v.Kv.Key, v.Kv.Value)
            s.doResolve(resolver.ResolveNowOptions{})
        }
    }
}
