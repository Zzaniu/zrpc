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
    "os"
    "os/signal"
    "time"
)

type (
    ServerConf2 struct {
        *rpc.ServerConf `yaml:"ServerConf"`
        Trace           ztracer.Trace `yaml:"Trace"`
    }

    AddServer struct {
        proto2.UnimplementedAddServerServer
        // 这里可以加个自己的 config, 然后在这里面注入 db 或者啥的, 会很方便
    }
)

var addServerConfigFile = flag.String("f", "serverCfg2.yaml", "the config file")

func main() {
    flag.Parse()
    cfg := ServerConf2{}

    configure.MustLoadCfg(*addServerConfigFile, &cfg)

    err := ztracer.SetJaegerTracerProvider(cfg.Trace)
    if err != nil {
        panic(err)
    }

    service := zrpc.MustNewServer(context.Background(), cfg, func(srv *grpc.Server) {
        addServer := &AddServer{}
        proto2.RegisterAddServerServer(srv, addServer)
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

func (a *AddServer) AddInt(ctx context.Context, req *proto2.AddIntRequest) (*proto2.AddIntReply, error) {
    fmt.Println("server 2")
    deadline, ok := ctx.Deadline()
    fmt.Println("deadline = ", deadline, ", ok = ", ok)
    time.Sleep(time.Millisecond * 50)
    return &proto2.AddIntReply{Message: req.Value1 + req.Value2}, nil
}
