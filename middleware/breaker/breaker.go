/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/1 11:03
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

package breaker

import (
    "context"
    "fmt"
    "github.com/Zzaniu/zrpc/tool/sre"
    "github.com/Zzaniu/zrpc/tool/zlog"
    "github.com/Zzaniu/zrpc/utils/errcode"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
    "sync"
)

type (
    Breaker interface {
        Allow() error
        MarkSuccess()
        MarkFailed()
    }

    Group interface {
        new() Breaker
        Get(string) Breaker
    }
    SreBreakerGroup struct {
        sync.RWMutex
        breakers map[string]Breaker
    }
)

func (g *SreBreakerGroup) Get(key string) Breaker {
    g.RLock()
    v, ok := g.breakers[key]
    if ok {
        g.RUnlock()
        return v
    }
    g.RUnlock()

    g.Lock()
    defer g.Unlock()
    v, ok = g.breakers[key]
    if ok {
        return v
    }
    v = g.new()
    g.breakers[key] = v
    return v
}

func (g *SreBreakerGroup) new() Breaker {
    breaker := sre.NewSreBreaker()
    return breaker
}

func NewSreBreakerGroup() Group {
    return &SreBreakerGroup{breakers: make(map[string]Breaker)}
}

func ExampleInterceptor(ctx context.Context, method string, req, reply interface{},
    cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    fmt.Printf("method = %v, type = %T\n", method, method)
    fmt.Printf("req = %v, type = %T\n", req, req)
    fmt.Printf("reply = %v, type = %T\n", reply, reply)
    fmt.Printf("opts = %v, type = %T\n", opts, opts)

    ctx = metadata.AppendToOutgoingContext(ctx, "token", "123456")
    err := invoker(ctx, method, req, reply, cc, opts...)
    fmt.Println("err = ", err)
    fmt.Println("reply = ", reply)
    return err
}

// ClientBreakInterceptor 客户端熔断器
func ClientBreakInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn,
    invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    // TODO 感觉客户端不需要熔断器啊
    return invoker(ctx, method, req, reply, cc, opts...)
}

// WithServerBreakerInterceptor rpc 服务端熔断器
func WithServerBreakerInterceptor() grpc.UnaryServerInterceptor {
    breakGroup := NewSreBreakerGroup()
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler) (interface{}, error) {
        // 跟gin一样, 首先判断熔断器是否打开, 打开则直接返回，未打开则允许通过并记录返回是否OK
        breaker := breakGroup.Get(info.FullMethod)
        e := breaker.Allow()
        if e == sre.ErrNotAllowed {
            zlog.Warnf("log [zrpc] sreBreaker dropped, %s", info.FullMethod)
            return nil, status.New(codes.Unavailable, e.Error()).Err()
        }

        // 执行请求
        resp, err := handler(ctx, req)

        // 熔断器标记该请求是否成功
        if errcode.Acceptable(err) {
            breaker.MarkSuccess()
        } else {
            breaker.MarkFailed()
        }

        return resp, err
    }
}
