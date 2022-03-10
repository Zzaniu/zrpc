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
	"time"
)

type (
	Server struct {
		server          *grpc.Server
		ctx             context.Context
		ServiceInstance *register.ServiceInstance
		register        register.IRegister
		serverOption    []grpc.ServerOption
	}

	RegisterServer func(server *grpc.Server)
	SOption        func(*[]grpc.ServerOption)
)

func MustNewServer(ctx context.Context, server rpc.Server, registerServer RegisterServer, serverOption ...grpc.ServerOption) *Server {
	srv := &Server{
		ctx:          ctx,
		register:     server.MustNewRegister(),
		serverOption: serverOption,
		ServiceInstance: register.NewServiceInstance(
			time.Now().Unix(),
			server.GetNamespace(),
			server.GetServiceName(),
			server.GetEndpoint()),
	}
	// span链路追踪、bbr自动降载、sre熔断器、recover拦截器
	WithServerOption(WithUnaryServerInterceptors(
		// TODO span链路追踪拦截器
		tracer.ServerTraceInterceptor,
		limiter.WithServerLimiterInterceptor(), // bbr 自动限流拦截器
		breaker.WithServerBreakerInterceptor(), // sre 弹性熔断拦截器
		recovery.UnaryRecoverInterceptor,       // recover 拦截器
	))(&srv.serverOption)
	srv.server = grpc.NewServer(srv.serverOption...)

	// 将服务注册进来
	registerServer(srv.server)

	return srv
}

// Serve 启动服务
func (s *Server) Serve() error {
	// 运行服务
	listen, err := net.Listen("tcp", s.ServiceInstance.Endpoint)
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

// BuildServerOption 构建服务端拦截器
func (s *Server) BuildServerOption() grpc.ServerOption {
	// TODO 创建拦截器，比如说那些熔断啊限流啊之类的东西了
	return nil
}

func WithServerOption(opts ...grpc.ServerOption) SOption {
	return func(options *[]grpc.ServerOption) {
		*options = append(*options, opts...)
	}
}

func WithUnaryServerInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(interceptors...)
}
