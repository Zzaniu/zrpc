/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/3 10:31
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

package balancer

import (
    "github.com/Zzaniu/zrpc/middleware/balancer/p2c"
    "google.golang.org/grpc/balancer"
    "google.golang.org/grpc/balancer/base"
)

// 注册全局 balancer
func init() {
    balancer.Register(newBalancer(p2c.Name, new(p2c.PickerBuilderP2c), base.Config{HealthCheck: true}))
}

func newBalancer(name string, builder base.PickerBuilder, config base.Config) balancer.Builder {
    return base.NewBalancerBuilder(name, builder, config)
}
