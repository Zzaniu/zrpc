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
	"fmt"
	"github.com/Zzaniu/zrpc/configure/rpc"
	_ "github.com/Zzaniu/zrpc/middleware/balancer"
	"github.com/Zzaniu/zrpc/middleware/balancer/p2c"
	"github.com/Zzaniu/zrpc/middleware/register"
	_ "github.com/Zzaniu/zrpc/middleware/resover"
	"github.com/Zzaniu/zrpc/middleware/resover/etcd"
	"github.com/Zzaniu/zrpc/middleware/timeout"
	"github.com/Zzaniu/zrpc/middleware/tracer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

type (
	IClient interface {
		Coon() *grpc.ClientConn
	}
	Client struct {
		coon      *grpc.ClientConn
		discovery register.IDiscovery
		opts      []grpc.DialOption
		target    string
	}

	COption func(*[]grpc.DialOption)
)

// MustNewClient 新建一个 Client 实例
func MustNewClient(rpcClient rpc.Client, serverName string, opts ...grpc.DialOption) IClient {
	client := &Client{
		discovery: rpcClient.MustNewDiscovery(),
		target:    rpcClient.GetTarget(serverName),
	}

	WithDialOption(opts...)(&client.opts)
	WithDialOption(Block())(&client.opts)                         // 阻塞直到链接建立成功
	WithDialOption(Insecure())(&client.opts)                      // 标志非安全的(不用HTTPS)
	WithDialOption(BalancerOption())(&client.opts)                // 负载均衡器(p2c)
	WithDialOption(WithDiscovery(client.discovery))(&client.opts) // 服务发现器
	WithDialOption(WithUnaryClientInterceptors(
		// TODO 链路追踪拦截器啊, 用来查看调用链哪个地方出错, 或者各链路是哪个地方耗时异常等
		tracer.ClientTraceInterceptor, // 链路追踪拦截器
		// breaker.ClientBreakInterceptor,            // 熔断拦截器(客户端真的需要熔断器吗？我感觉是不需要啊)
		timeout.TimeoutInterceptor(time.Second*3), // 请求超时拦截器
		// 	TODO 重试拦截器, 如果请求失败就重试个两三次之类的
	))(&client.opts)
	// 连接超时
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFunc()
	coon, err := grpc.DialContext(ctx, client.target, client.opts...)
	if err != nil {
		panic(fmt.Sprintf("连接超时, serverName = %v, target = %v, err = %v\n", serverName, client.target, err))
	}

	return &Client{coon: coon}
}

func (c *Client) Coon() *grpc.ClientConn {
	return c.coon
}

func BalancerOption() grpc.DialOption {
	return grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, p2c.Name))
}

func WithDialOption(opts ...grpc.DialOption) COption {
	return func(options *[]grpc.DialOption) {
		*options = append(*options, opts...)
	}
}

// WithDiscovery 注册非全局的服务发现，此优先级高于全局注册
func WithDiscovery(discoverer register.IDiscovery) grpc.DialOption {
	return grpc.WithResolvers(etcd.NewResolverBuilderEtcd(discoverer))
}

func Block() grpc.DialOption {
	return grpc.WithBlock()
}

func Insecure() grpc.DialOption {
	return grpc.WithTransportCredentials(insecure.NewCredentials())
}

// WithUnaryClientInterceptors 按顺序加载拦截器
// WithChainUnaryInterceptor 是一个按照顺序来的
// WithUnaryInterceptor 总是在最前面
func WithUnaryClientInterceptors(interceptors ...grpc.UnaryClientInterceptor) grpc.DialOption {
	return grpc.WithChainUnaryInterceptor(interceptors...)
}
