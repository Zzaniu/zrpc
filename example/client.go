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
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "time"
)

type (
    ClientConf struct {
        *rpc.ClientConf `yaml:"ClientConf"`
        UserServerName  string        `yaml:"UserServerName"`
        AddServerName   string        `yaml:"AddServerName"`
        Trace           ztracer.Trace `yaml:"Trace"`
    }
)

var clientConfigFile = flag.String("f", "clientCfg.yaml", "the config file")

func main() {
    flag.Parse()
    cfg := ClientConf{}
    configure.MustLoadCfg(*clientConfigFile, &cfg)

    err := ztracer.SetJaegerTracerProvider(cfg.Trace)
    if err != nil {
        panic(err)
    }

    // target := "discovery://172.18.2.249:20000,172.18.2.249:20002,172.18.2.249:20004/Dev/user.rpc"
    userClient := zrpc.MustNewClient(cfg, cfg.UserServerName)
    fmt.Println("userClient.GetTarget() = ", userClient.GetTarget())
    addClientConn := zrpc.MustNewClientConn(cfg, cfg.AddServerName)
    UserRpc := proto2.NewGreeterClient(userClient.Coon())
    AddRpc := proto2.NewAddServerClient(addClientConn)

    for {
        go func() {
            ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*2000)
            deadline, ok := ctx.Deadline()
            fmt.Println("deadline = ", deadline, ", ok = ", ok)
            res, err := UserRpc.SayHello(ctx, &proto2.HelloRequest{Name: "小可爱"})
            defer cancelFunc()
            if err != nil {
                s, ok := status.FromError(err)
                if ok {
                    if s.Code() == codes.DeadlineExceeded {
                        fmt.Println("超时了, err = ", s.Message())
                    } else if s.Code() == codes.Internal {
                        fmt.Println("s.Message() = ", s.Message())
                    } else if s.Code() == codes.Unavailable {
                        fmt.Println("熔断了")
                    }
                } else {
                    panic(err)
                }
            } else {
                fmt.Println("res.Message = ", res.Message)
            }
        }()

        go func() {
            res2, err := AddRpc.AddInt(context.Background(), &proto2.AddIntRequest{Value2: 2, Value1: 1})

            if err != nil {
                s, ok := status.FromError(err)
                if ok {
                    if s.Code() == codes.DeadlineExceeded {
                        fmt.Println("超时了, err = ", s.Message())
                    } else if s.Code() == codes.Internal {
                        fmt.Println("s.Message() = ", s.Message())
                    }
                } else {
                    panic(err)
                }
            } else {
                fmt.Println("res2.Message = ", res2.Message)
            }
        }()
        // time.Sleep(time.Millisecond * 5000)
        break
    }
    time.Sleep(time.Second * 5)
}
