/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/2 11:43
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

package register

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-basic/uuid"
	"strings"
)

type (
	// IRegister 服务注册
	IRegister interface {
		Register(ctx context.Context, service *ServiceInstance) error
		Deregister(ctx context.Context, service *ServiceInstance) error
	}

	// IDiscovery 服务发现
	IDiscovery interface {
		GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
		// Watch creates a watcher according to the service name.
		Watch(ctx context.Context, serviceName string) (IWatcher, error)
	}

	IWatcher interface {
		// Next returns services in the following two cases:
		// 1.the first time to watch and the service instance list is not empty.
		// 2.any service instance changes found.
		// if the above two conditions are not met, it will block until context deadline exceeded or canceled
		Next() ([]*ServiceInstance, error)
		// Stop close the watcher.
		Stop() error
	}

	ServiceInstance struct {
		Name     string `json:"name"`
		Key      string `json:"key"`
		Endpoint string `json:"endpoint"`
		// 	TODO 后续可以考虑加入权重、版本等信息？
	}
)

func NewServiceInstance(id int64, nameSpace, name, endpoint string) *ServiceInstance {
	return &ServiceInstance{
		Name:     fmt.Sprintf("%s/%s", nameSpace, name),
		Key:      fmt.Sprintf("%s/%s/%d/%s", nameSpace, name, id, uuid.New()),
		Endpoint: endpoint,
	}
}

func (s *ServiceInstance) GetServerName() string {
	return strings.Split(s.Name, "/")[1]
}

func Marshal(s *ServiceInstance) (string, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Unmarshal(data []byte) (s *ServiceInstance, err error) {
	s = &ServiceInstance{}
	err = json.Unmarshal(data, s)
	return
}
