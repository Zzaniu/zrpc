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
	_ "github.com/Zzaniu/zrpc/middleware/resolver"
	"github.com/Zzaniu/zrpc/middleware/resolver/etcd"
	"github.com/Zzaniu/zrpc/middleware/timeout"
	"github.com/Zzaniu/zrpc/middleware/tracer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

type (
	Client struct {
		discovery  register.IDiscovery
		serverName string
		target     string
		option     clientOption
	}

	clientOption struct {
		opts              []grpc.DialOption
		clientInterceptor []grpc.UnaryClientInterceptor
	}

	COption func(*clientOption)
)

func (c *Client) Coon() *grpc.ClientConn {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
	defer cancelFunc()
	coon, err := grpc.DialContext(ctx, c.target, c.option.opts...)
	if err != nil {
		panic(fmt.Sprintf("连接超时, serverName = %v, target = %v, err = %v\n", c.serverName, c.target, err))
	}
	return coon
}

func (c *Client) GetTarget() string {
	return c.target
}

func MustNewClient(rpcClient rpc.Client, serverName string, opts ...COption) *Client {
	client := &Client{
		discovery:  rpcClient.MustNewDiscovery(),
		serverName: serverName,
		target:     rpcClient.BuildTarget(serverName),
		option: clientOption{
			clientInterceptor: []grpc.UnaryClientInterceptor{
				tracer.ClientTraceInterceptor, // 链路追踪拦截器
				timeout.TimeoutInterceptor(time.Millisecond * time.Duration(rpcClient.TimeOut())), // 请求超时拦截器
				// breaker.ClientBreakInterceptor,            // 熔断拦截器(客户端真的需要熔断器吗？我感觉是不需要啊)
				// TODO 重试拦截器, 如果请求失败就重试个两三次之类的
			},
		},
	}

	options := []COption{
		// TODO 后续可以考虑是否放开 https
		WithDialOption(Insecure()),                      // 标志非安全的(不用HTTPS)
		WithDialOption(balancerOption()),                // 负载均衡器(p2c)
		WithDialOption(withDiscovery(client.discovery)), // 服务发现器
	}
	if !rpcClient.NoBlock() {
		// 阻塞直到链接建立成功
		options = append(options, WithDialOption(Block()))
	}

	if len(opts) > 0 {
		options = append(options, opts...) // 在这里注入 clientInterceptor、grpc.DialOption 等
	}

	for _, o := range options {
		o(&client.option)
	}

	// 添加 链路追踪拦截器、请求超时拦截器， 以及用户传入的拦截器
	WithDialOption(withUnaryClientInterceptors(client.option.clientInterceptor...))(&client.option)
	return client
}

// MustNewClientConn 新建一个 Client 实例
func MustNewClientConn(rpcClient rpc.Client, serverName string, opts ...COption) *grpc.ClientConn {
	client := MustNewClient(rpcClient, serverName, opts...)
	return client.Coon()
}

// BalancerOption 负载均衡
func balancerOption() grpc.DialOption {
	return grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingPolicy":"%s"}`, p2c.Name))
}

func WithDialOption(opts ...grpc.DialOption) COption {
	return func(option *clientOption) {
		option.opts = append(option.opts, opts...)
	}
}

func WithClientInterceptor(clientInterceptor ...grpc.UnaryClientInterceptor) COption {
	return func(option *clientOption) {
		option.clientInterceptor = append(option.clientInterceptor, clientInterceptor...)
	}
}

// withDiscovery 注册非全局的服务发现，此优先级高于全局注册
func withDiscovery(discoverer register.IDiscovery) grpc.DialOption {
	return grpc.WithResolvers(etcd.NewResolverBuilderEtcd(discoverer))
}

func Block() grpc.DialOption {
	return grpc.WithBlock()
}

func Insecure() grpc.DialOption {
	return grpc.WithTransportCredentials(insecure.NewCredentials())
}

// withUnaryClientInterceptors 按顺序加载拦截器
// WithChainUnaryInterceptor 是一个按照顺序来的
// WithUnaryInterceptor 总是在最前面
func withUnaryClientInterceptors(interceptors ...grpc.UnaryClientInterceptor) grpc.DialOption {
	return grpc.WithChainUnaryInterceptor(interceptors...)
}
