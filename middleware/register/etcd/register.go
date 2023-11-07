/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/2 11:43
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
    "github.com/Zzaniu/zrpc/middleware/register"
    "go.etcd.io/etcd/api/v3/mvccpb"
    clientv3 "go.etcd.io/etcd/client/v3"
    "math/rand"
    "strings"
    "time"
)

type (
    RegisterEtcd struct {
        client *clientv3.Client
        lease  clientv3.Lease
        kv     clientv3.KV
        opt    option
    }

    option struct {
        registerServiceUri string
        userName           string
        passWord           string
        ttl                int64
        maxRetry           int
        ctx                context.Context
        cancel             context.CancelFunc
    }

    Option func(*option)
)

func WithTTL(ttl int64) Option {
    return func(o *option) { o.ttl = ttl }
}

func WithMaxRetry(maxRetry int) Option {
    return func(o *option) { o.maxRetry = maxRetry }
}

func WithRegisterServiceUri(registerServiceUri string) Option {
    return func(o *option) { o.registerServiceUri = registerServiceUri }
}

func WithCancelCtx(ctx context.Context, cancel context.CancelFunc) Option {
    return func(o *option) {
        o.ctx = ctx
        o.cancel = cancel
    }
}

func WithUsername(userName string) Option {
    return func(o *option) {
        o.userName = userName
    }
}

func WithPassword(passWord string) Option {
    return func(o *option) {
        o.passWord = passWord
    }
}

func NewRegisterEtcd(options ...Option) (*RegisterEtcd, error) {
    opt := option{
        ttl:      10,
        maxRetry: 5,
        ctx:      context.Background(),
    }
    for _, o := range options {
        o(&opt)
    }
    etcdClient, err := clientv3.New(clientv3.Config{
        Endpoints:   strings.Split(opt.registerServiceUri, ","),
        DialTimeout: 1 * time.Second,
        Username:    opt.userName,
        Password:    opt.passWord,
    })
    if err != nil {
        return nil, err
    }
    return &RegisterEtcd{client: etcdClient, opt: opt, kv: clientv3.NewKV(etcdClient)}, nil

}

// Register 注册
func (r *RegisterEtcd) Register(ctx context.Context, service *register.ServiceInstance) error {
    if r.lease != nil {
        _ = r.lease.Close()
    }
    r.lease = clientv3.NewLease(r.client)
    value, err := register.Marshal(service)
    if err != nil {
        return err
    }
    leaseID, err := r.registerWithKV(ctx, service.Key, value)
    if err != nil {
        return err
    }

    go r.heartBeat(r.opt.ctx, leaseID, service.Key, value)
    return nil
}

// Deregister 取消注册
func (r *RegisterEtcd) Deregister(ctx context.Context, service *register.ServiceInstance) error {
    defer func() {
        if r.lease != nil {
            _ = r.lease.Close()
        }
        r.opt.cancel()
    }()
    _, err := r.client.Delete(ctx, service.Key)
    return err
}

// registerWithKV 往 etcd 写一个 key
func (r *RegisterEtcd) registerWithKV(ctx context.Context, key, value string) (clientv3.LeaseID, error) {
    grant, err := r.lease.Grant(ctx, r.opt.ttl)
    if err != nil {
        return 0, err
    }
    _, err = r.client.Put(ctx, key, value, clientv3.WithLease(grant.ID))
    if err != nil {
        return 0, err
    }
    return grant.ID, nil
}

// heartBeat 就相当于 KeepAlive
func (r *RegisterEtcd) heartBeat(ctx context.Context, leaseID clientv3.LeaseID, key string, value string) {
    curLeaseID := leaseID
    kac, err := r.client.KeepAlive(ctx, leaseID)
    if err != nil {
        curLeaseID = 0
    }
    rand.Seed(time.Now().Unix())

    for {
        if curLeaseID == 0 {
            // try to registerWithKV
            var retreat []int
            for retryCnt := 0; retryCnt < r.opt.maxRetry; retryCnt++ {
                if ctx.Err() != nil {
                    return
                }
                // 这个必须设置为有缓存的，否则可能会造成 goroutine 泄露
                idChan := make(chan clientv3.LeaseID, 1)
                errChan := make(chan error, 1)
                cancelCtx, cancel := context.WithCancel(ctx)
                go func() {
                    defer cancel()
                    id, registerErr := r.registerWithKV(cancelCtx, key, value)
                    if registerErr != nil {
                        errChan <- registerErr
                    } else {
                        idChan <- id
                    }
                }()

                select {
                case <-time.After(3 * time.Second):
                    cancel()
                    continue
                case <-errChan:
                    continue
                case curLeaseID = <-idChan:
                }

                kac, err = r.client.KeepAlive(ctx, curLeaseID)
                if err == nil {
                    break
                }
                retreat = append(retreat, 1<<retryCnt)
                time.Sleep(time.Duration(retreat[rand.Intn(len(retreat))]) * time.Second)
            }
            // TODO: if retry failed, do something with this error
            if _, ok := <-kac; !ok {
                // retry failed
                return
            }
        }

        select {
        case _, ok := <-kac:
            if !ok {
                if ctx.Err() != nil {
                    // channel closed due to context cancel
                    return
                }
                // need to retry registration
                curLeaseID = 0
            }
        case <-r.opt.ctx.Done():
            return
        }
    }
}

// GetService 从 etcd 中获取服务
func (r *RegisterEtcd) GetService(ctx context.Context, serviceName string) ([]*register.ServiceInstance, error) {
    resp, err := r.kv.Get(ctx, serviceName, clientv3.WithPrefix())
    if err != nil {
        return nil, err
    }

    return getServiceInstance(resp.Kvs, serviceName)
}

func getServiceInstance(kvs []*mvccpb.KeyValue, serviceName string) ([]*register.ServiceInstance, error) {
    items := make([]*register.ServiceInstance, 0, len(kvs))
    for _, kv := range kvs {
        si, err := register.Unmarshal(kv.Value)
        if err != nil {
            return nil, err
        }
        if si.Name != serviceName {
            continue
        }
        items = append(items, si)
    }
    return items, nil
}

// Watch 监测服务
func (r *RegisterEtcd) Watch(ctx context.Context, serviceName string) (register.IWatcher, error) {
    return newWatcher(ctx, serviceName, r.client)
}
