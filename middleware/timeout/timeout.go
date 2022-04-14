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
        if timeout <= 0 {
            return invoker(ctx, method, req, reply, cc, opts...)
        }

        ctx, cancel := context.WithTimeout(ctx, timeout)
        defer cancel()
        // grpc 已经实现了超时控制，所以这里只要把ctx替换成带超时的ctx就行了
        return invoker(ctx, method, req, reply, cc, opts...)
    }
}
