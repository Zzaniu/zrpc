/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/2/28 16:37
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

package zrpc

import (
    "context"
    "github.com/Zzaniu/zrpc/configure/rpc"
    "github.com/Zzaniu/zrpc/middleware/breaker"
    "github.com/Zzaniu/zrpc/middleware/limiter"
    "github.com/Zzaniu/zrpc/middleware/recovery"
    "github.com/Zzaniu/zrpc/middleware/register"
    "github.com/Zzaniu/zrpc/middleware/tracer"
    "google.golang.org/grpc"
    "net"
    "strings"
    "time"
)

type (
    Server struct {
        server          *grpc.Server
        ctx             context.Context
        ServiceInstance *register.ServiceInstance
        register        register.IRegister
        option          serverOption
    }

    serverOption struct {
        opts              []grpc.ServerOption
        serverInterceptor []grpc.UnaryServerInterceptor
        serveEndpoint     string
    }

    RegisterServer func(server *grpc.Server)
    SOption        func(*serverOption)
)

func MustNewServer(ctx context.Context, server rpc.Server, registerServer RegisterServer, serverOptions ...SOption) *Server {
    srv := &Server{
        ctx:      ctx,
        register: server.MustNewRegister(),
        option: serverOption{
            serverInterceptor: []grpc.UnaryServerInterceptor{
                tracer.ServerTraceInterceptor,          // openTelemetry 链路追踪拦截器
                limiter.WithServerLimiterInterceptor(), // bbr 自动限流拦截器
                breaker.WithServerBreakerInterceptor(), // sre 弹性熔断拦截器
                recovery.UnaryRecoverInterceptor,       // recover 拦截器
            },
            serveEndpoint: server.GetEndpoint(),
        },
        ServiceInstance: register.NewServiceInstance(
            time.Now().Unix(),
            server.GetNamespace(),
            server.GetServiceName(),
            server.GetEndpoint()),
    }

    for _, o := range serverOptions {
        o(&srv.option) // 在这里注入 serverInterceptor、grpc.ServerOption 等
    }

    // 添加 openTelemetry 链路追踪、bbr自动降载、sre熔断器、recover拦截器, 以及用户传入的拦截器
    WithServerOption(withUnaryServerInterceptors(srv.option.serverInterceptor...))(&srv.option)

    srv.server = grpc.NewServer(srv.option.opts...)

    // 将服务注册进来
    registerServer(srv.server)

    return srv
}

// Serve 启动服务
func (s *Server) Serve() error {
    // 运行服务
    listen, err := net.Listen("tcp", s.option.serveEndpoint)
    if err != nil {
        return err
    }
    errChan := make(chan error)
    go func() {
        if err := s.server.Serve(listen); err != nil {
            errChan <- err
        }
    }()
    // 注册会自动保持心跳
    if err := s.register.Register(s.ctx, s.ServiceInstance); err != nil {
        return err
    }
    return <-errChan
}

// StopServe 停止服务
func (s *Server) StopServe() {
    if s.server != nil {
        _ = s.register.Deregister(context.Background(), s.ServiceInstance)
        // 优雅停止
        s.server.GracefulStop()
        // // 立即停止
        // s.server.Stop()
    }
}

// ServerEndpoint 服务真正运行的 endpoint
func (s *Server) ServerEndpoint() string {
    return s.option.serveEndpoint
}

// WithServerOption 通过 grpc.ServerOption 生成 SOption
func WithServerOption(opts ...grpc.ServerOption) SOption {
    return func(option *serverOption) {
        option.opts = append(option.opts, opts...)
    }
}

// WithServerInterceptor 生成服务端拦截器
func WithServerInterceptor(serverInterceptor ...grpc.UnaryServerInterceptor) SOption {
    return func(option *serverOption) {
        option.serverInterceptor = append(option.serverInterceptor, serverInterceptor...)
    }
}

// WithServeZero 设置服务启动运行在 0.0.0.0
func WithServeZero() SOption {
    return func(option *serverOption) {
        option.serveEndpoint = "0.0.0.0:" + strings.Split(option.serveEndpoint, ":")[1]
    }
}

// withUnaryServerInterceptors 生成顺序的服务端拦截器
func withUnaryServerInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.ServerOption {
    return grpc.ChainUnaryInterceptor(interceptors...)
}
