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
    "github.com/Zzaniu/tool/zlog"
    "strings"
    "sync"
    "time"

    "github.com/go-redis/redis"
)

type (
    Redis struct {
        URI  string `yaml:"URI"`
        Auth string `yaml:"Auth"`
        Db   int    `yaml:"Db"`
        Type string `yaml:"Type"`
    }

    Option struct {
        dialTimeout  time.Duration
        readTimeout  time.Duration
        writeTimeout time.Duration
    }

    Opts func(*Option)
)

var (
    cacheOnce    sync.Once
    cache        *redis.Client
    clusterCache *redis.ClusterClient
)

func WithDialTimeout(dialTimeout time.Duration) Opts {
    return func(opt *Option) {
        opt.dialTimeout = dialTimeout
    }
}

func WithReadTimeout(readTimeout time.Duration) Opts {
    return func(opt *Option) {
        opt.readTimeout = readTimeout
    }
}

func WithWriteTimeout(writeTimeout time.Duration) Opts {
    return func(opt *Option) {
        opt.writeTimeout = writeTimeout
    }
}

func (rds *Redis) Init(opts ...Opts) {
    cacheOnce.Do(func() {
        opt := Option{
            dialTimeout:  50 * time.Millisecond, // 设置连接超时
            readTimeout:  50 * time.Millisecond, // 设置读取超时
            writeTimeout: 50 * time.Millisecond, // 设置写入超时
        }

        for _, o := range opts {
            o(&opt)
        }

        switch rds.Type {
        case "cluster":
            clusterCache = redis.NewClusterClient(&redis.ClusterOptions{
                Addrs:        strings.Split(rds.URI, ","),
                Password:     rds.Auth,
                DialTimeout:  opt.dialTimeout,  // 设置连接超时
                ReadTimeout:  opt.readTimeout,  // 设置读取超时
                WriteTimeout: opt.writeTimeout, // 设置写入超时
                PoolSize:     1,
                MinIdleConns: 0,
            })
            _, err := clusterCache.Ping().Result()
            if err != nil {
                zlog.Fatalf("redis cluster ping failed: %v", err)
            }
        default:
            cache = redis.NewClient(&redis.Options{
                Addr:         rds.URI,
                Password:     rds.Auth,         // no password set
                DialTimeout:  opt.dialTimeout,  // 设置连接超时
                ReadTimeout:  opt.readTimeout,  // 设置读取超时
                WriteTimeout: opt.writeTimeout, // 设置写入超时
                DB:           rds.Db,           // use default DB
            })
            _, err := cache.Ping().Result()
            if err != nil {
                zlog.Fatalf("redis ping failed: %v", err)
            }
        }
    })
}

// GetCache 返回缓存实例
func (rds *Redis) GetCache() redis.Cmdable {
    switch rds.Type {
    case "cluster":
        return clusterCache
    default:
        return cache
    }
}
