/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/2/28 16:29
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

package redis

import (
	"strings"
	"sync"
	"time"
	"zrpc/tool/xlog"

	"github.com/go-redis/redis"
)

type (
	Redis struct {
		URI  string `yaml:"URI"`
		Auth string `yaml:"Auth"`
		Db   int    `yaml:"Db"`
		Type string `yaml:"Type"`
	}
)

var (
	cacheOnce    sync.Once
	cache        *redis.Client
	clusterCache *redis.ClusterClient
	redisClient  *Redis
)

func (rds *Redis) Init() {
	cacheOnce.Do(func() {
		switch rds.Type {
		case "cluster":
			clusterCache = redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:        strings.Split(rds.URI, ","),
				Password:     rds.Auth,
				DialTimeout:  50 * time.Millisecond, // 设置连接超时
				ReadTimeout:  50 * time.Millisecond, // 设置读取超时
				WriteTimeout: 50 * time.Millisecond, // 设置写入超时
				PoolSize:     1,
				MinIdleConns: 0,
			})
			_, err := clusterCache.Ping().Result()
			if err != nil {
				xlog.XLog.Fatalf("redis cluster ping failed: %v", err)
			}
		default:
			cache = redis.NewClient(&redis.Options{
				Addr:     rds.URI,
				Password: rds.Auth, // no password set
				DB:       rds.Db,   // use default DB
			})
			_, err := cache.Ping().Result()
			if err != nil {
				xlog.XLog.Fatalf("redis ping failed: %v", err)
			}
		}
	})
}

// GetCache 返回缓存实例
func GetCache() redis.Cmdable {
	switch redisClient.Type {
	case "cluster":
		return clusterCache
	default:
		return cache
	}
}
