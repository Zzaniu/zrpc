/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/7 14:26
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

package timeout

import (
    "context"
    "google.golang.org/grpc"
    "time"
)

// TimeoutInterceptor 超时拦截器
func TimeoutInterceptor(timeout time.Duration) grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn,
        invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        // 如果当前没有设置超时时间且默认超时时间大于0, 则给个默认的超时时间. 以当前传入的超时时间为主
        if _, ok := ctx.Deadline(); !ok && timeout > 0 {
            ctx2, cancel := context.WithTimeout(ctx, timeout)
            defer cancel()
            ctx = ctx2
        }
        // grpc 已经实现了超时控制，所以这里只要把ctx替换成带超时的ctx就行了
        return invoker(ctx, method, req, reply, cc, opts...)
    }
}
