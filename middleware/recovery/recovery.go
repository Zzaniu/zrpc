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
	"fmt"
	"github.com/Zzaniu/zrpc/tool/zlog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"runtime"
)

// UnaryRecoverInterceptor TODO 没有堆栈信息
func UnaryRecoverInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (resp interface{}, err error) {
	defer handleCrash(func(r interface{}) {
		err = toPanicError(r)
	})

	return handler(ctx, req)
}

func handleCrash(handler func(interface{})) {
	if r := recover(); r != nil {
		handler(r)
	}
}

func toPanicError(r interface{}) error {
	zlog.Errorf("\n%v", getWhere(r))
	return status.Errorf(codes.Internal, "panic: %v", r)
}

func getWhere(r interface{}) string {
	_, file, line, ok := runtime.Caller(5)
	if ok {
		return fmt.Sprintf("    %v:%v %v\n", file, line, r)
	}
	return fmt.Sprintf("%v\n", r)
}
