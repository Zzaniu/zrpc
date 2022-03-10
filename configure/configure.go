/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/2/28 15:44
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

package configure

import (
	"github.com/Zzaniu/zrpc/tool/xlog"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

const (
	Dev    = "Dev"
	Test   = "Test"
	Online = "Online"
)

type (
	Jwt struct {
		Secret              string `yaml:"Secret"`
		TokenExpireDuration int    `yaml:"TokenExpireDuration"`
	}

	EtcdConf struct {
		Hosts string `yaml:"Hosts"`
		User  string `yaml:"User"`
		Pass  string `yaml:"Pass"`
	}
)

// HasEtcd 是否有 Etcd (目前还没啥用啊，因为目前是写死了支持etcd)
func (e *EtcdConf) HasEtcd() bool {
	return len(e.Hosts) > 0
}

func MustLoadCfg(path string, config interface{}) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		xlog.XLog.Fatalf("read cfg file fail: %v", err)
	}
	err = yaml.Unmarshal(content, config)
	if err != nil {
		xlog.XLog.Fatalf("read cfg file fail: %v", err)
	}
}
