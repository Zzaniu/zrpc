/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/7 14:46
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

package recovery

import (
    "context"
    "github.com/Zzaniu/tool/zlog"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "runtime/debug"
    "unsafe"
)

// UnaryRecoverInterceptor 如果报错, 打印堆栈信息
func UnaryRecoverInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler) (resp interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            debugInfo := debug.Stack()
            zlog.Errorf("err: %v\n%v", r, *(*string)(unsafe.Pointer(&debugInfo)))
            err = status.Errorf(codes.Internal, "panic: %v", r)
        }
    }()

    return handler(ctx, req)
}
