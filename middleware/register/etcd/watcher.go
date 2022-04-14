/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/2 16:23
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
    clientv3 "go.etcd.io/etcd/client/v3"
)

var _ register.IWatcher = &Watcher{}

type Watcher struct {
    ctx         context.Context
    cancel      context.CancelFunc
    watchChan   clientv3.WatchChan
    watcher     clientv3.Watcher
    kv          clientv3.KV
    first       bool
    serviceName string
}

func newWatcher(ctx context.Context, name string, client *clientv3.Client) (*Watcher, error) {
    w := &Watcher{
        watcher:     clientv3.NewWatcher(client),
        kv:          clientv3.NewKV(client),
        first:       true,
        serviceName: name,
    }
    w.ctx, w.cancel = context.WithCancel(ctx)
    // 在这里 Watch   在 Next 取
    w.watchChan = w.watcher.Watch(w.ctx, w.serviceName, clientv3.WithPrefix(), clientv3.WithRev(0))
    // 告知客户端当前最新的集群状态, 会立即返回
    err := w.watcher.RequestProgress(context.Background())
    if err != nil {
        return nil, err
    }
    return w, nil
}

// Next 分两种情况
// 1. 第一次是立刻返回当前集群状态
// 2. 服务有更新的话，返回，无更新则阻塞
func (w *Watcher) Next() ([]*register.ServiceInstance, error) {
    // 第一次是 RequestProgress 立刻获取集群最新状态
    if w.first {
        w.first = false
        return w.getInstance()
    }

    select {
    // 取消了
    case <-w.ctx.Done():
        return nil, w.ctx.Err()
    // 说明服务有变动，有服务下线或者服务上线
    case <-w.watchChan:
        return w.getInstance()
    }
}

// Stop 停止监听
func (w *Watcher) Stop() error {
    w.cancel()
    return w.watcher.Close()
}

// getInstance 获取所有的服务
func (w *Watcher) getInstance() ([]*register.ServiceInstance, error) {
    // 通过前缀获取etcd中注册的服务
    resp, err := w.kv.Get(w.ctx, w.serviceName, clientv3.WithPrefix())
    if err != nil {
        return nil, err
    }
    // 获取所有的服务，返回一个切片
    return getServiceInstance(resp.Kvs, w.serviceName)
}
