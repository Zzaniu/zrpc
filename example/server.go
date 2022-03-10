/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/4 9:33
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

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Zzaniu/zrpc"
	"github.com/Zzaniu/zrpc/configure"
	"github.com/Zzaniu/zrpc/configure/rpc"
	proto2 "github.com/Zzaniu/zrpc/example/proto"
	"github.com/Zzaniu/zrpc/tool/ztracer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"os/signal"
	"time"
)

type (
	ServerConf struct {
		*rpc.ServerConf `yaml:"ServerConf"`
		Trace           ztracer.Trace `yaml:"Trace"`
	}

	GreeterServer struct {
		proto2.UnimplementedGreeterServer
		// 这里可以加个自己的 config, 然后在这里面注入 db 或者啥的, 会很方便
		AddServer proto2.AddServerClient
	}
)

var serverConfigFile = flag.String("f", "serverCfg.yaml", "the config file")

func main() {
	flag.Parse()
	cfg := ServerConf{}

	configure.MustLoadCfg(*serverConfigFile, &cfg)

	err := ztracer.SetJaegerTracerProvider(cfg.Trace)
	if err != nil {
		panic(err)
	}

	// test start

	conf := &rpc.ClientConf{
		Model: cfg.Model,
		EtcdConf: configure.EtcdConf{
			Hosts: cfg.Hosts,
		},
	}
	client := zrpc.MustNewClient(conf, "add.rpc")
	AddRpc := proto2.NewAddServerClient(client.Coon())

	// test end

	service := zrpc.MustNewServer(context.Background(), cfg, func(srv *grpc.Server) {
		greeterServer := &GreeterServer{
			AddServer: AddRpc,
		}
		proto2.RegisterGreeterServer(srv, greeterServer)
	})

	go func() {
		fmt.Println("服务启动")
		err := service.Serve()
		if err != nil {
			panic(err)
		}
	}()

	var c = make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)
	<-c
	fmt.Println("服务终止")
	service.StopServe()
}

func (g *GreeterServer) SayHello(ctx context.Context, req *proto2.HelloRequest) (*proto2.HelloReply, error) {
	fmt.Println("server 2")
	time.Sleep(time.Millisecond * 30)
	deadline, ok := ctx.Deadline()
	fmt.Println("deadline = ", deadline, ", ok = ", ok)
	addInt, err := g.AddServer.AddInt(ctx, &proto2.AddIntRequest{Value1: 1, Value2: 3})
	if err != nil {
		s, ok := status.FromError(err)
		if ok {
			if s.Code() == codes.Internal {
				fmt.Println("内部错误")
				return nil, status.New(codes.Internal, "系统不小心打了个盹，请稍后重试").Err()
			} else if s.Code() == codes.DeadlineExceeded {
				fmt.Println("超时了")
				return nil, status.New(codes.DeadlineExceeded, s.Message()).Err()
			} else if s.Code() == codes.Canceled {
				fmt.Println("客户端取消了")
				return nil, status.New(codes.Canceled, s.Message()).Err()
			}
		} else {
			return nil, status.New(codes.Internal, "系统不小心打了个盹，请稍后重试").Err()
		}
	}
	fmt.Println("addInt.Message = ", addInt.Message)
	return &proto2.HelloReply{Message: "你好啊，" + req.Name}, nil
}
