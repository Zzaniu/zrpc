/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/4 10:01
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

package rpc

import (
	"context"
	"fmt"
	"github.com/Zzaniu/zrpc/configure"
	"github.com/Zzaniu/zrpc/middleware/register"
	"github.com/Zzaniu/zrpc/middleware/register/etcd"
)

type (
	ServerConf struct {
		configure.EtcdConf `yaml:"Etcd"`
		ServerName         string `yaml:"ServerName"`
		Endpoint           string `yaml:"Endpoint"`
		Model              string `yaml:"Model"`
	}

	ClientConf struct {
		configure.EtcdConf `yaml:"Etcd"`
		nonBlock           bool   `yaml:"NonBlock"`
		model              string `yaml:"Model"`
	}

	Server interface {
		HasEtcd() bool
		GetNamespace() string
		GetServiceName() string
		GetEndpoint() string
		MustNewRegister() register.IRegister
	}

	Client interface {
		HasEtcd() bool
		GetTarget(string) string
		NoBlock() bool
		MustNewDiscovery() register.IDiscovery
	}
)

// GetNamespace 返回 namespace
func (s *ServerConf) GetNamespace() string {
	return s.Model
}

// GetServiceName 返回服务名
func (s *ServerConf) GetServiceName() string {
	return s.ServerName
}

// MustNewRegister new 一个注册器
func (s *ServerConf) MustNewRegister() register.IRegister {
	registerEtcd, err := etcd.NewRegisterEtcd(
		etcd.WithTTL(5),
		etcd.WithRegisterServiceUri(s.Hosts),
		etcd.WithCancelCtx(context.WithCancel(context.Background())),
	)
	if err != nil {
		panic(err)
	}
	return registerEtcd
}

// GetEndpoint 返回服务的监听IP与PORT
func (s *ServerConf) GetEndpoint() string {
	return s.Endpoint
}

// GetTarget 返回一个用来做服务发现的 target
func (c *ClientConf) GetTarget(serverName string) string {
	return fmt.Sprintf("discovery://%s/%s/%s", c.EtcdConf.Hosts, c.model, serverName)
}

// MustNewDiscovery new 一个 Discovery
func (c *ClientConf) MustNewDiscovery() register.IDiscovery {
	discovery, err := etcd.NewRegisterEtcd(
		etcd.WithTTL(5),
		etcd.WithRegisterServiceUri(c.Hosts),
		etcd.WithCancelCtx(context.WithCancel(context.Background())),
	)
	if err != nil {
		panic(err)
	}
	return discovery
}

func (c *ClientConf) NoBlock() bool {
	return c.nonBlock
}
