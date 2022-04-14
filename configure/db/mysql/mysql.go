/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/2/28 16:49
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

package mysql

import (
    "fmt"
    "github.com/Zzaniu/zrpc/tool/zlog"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
    "sync"
    "time"
)

type (
    Mysql struct {
        Host     string `yaml:"Host"`
        Port     int    `yaml:"Port"`
        Username string `yaml:"Username"`
        Password string `yaml:"Password"`
        Database string `yaml:"Database"`
    }

    Option struct {
        config          gorm.Config
        MaxIdleConns    int
        MaxOpenConns    int
        ConnMaxIdleTime time.Duration
        ConnMaxLifetime time.Duration
    }

    Opts func(*Option)
)

var (
    mysqlOnce sync.Once
    dbConn    gorm.DB
)

func WithGormOption(config gorm.Config) Opts {
    return func(opt *Option) {
        opt.config = config
    }
}

func WithMaxIdleConns(maxIdleConns int) Opts {
    return func(opt *Option) {
        opt.MaxIdleConns = maxIdleConns
    }
}

func WithMaxOpenConns(maxOpenConns int) Opts {
    return func(opt *Option) {
        opt.MaxOpenConns = maxOpenConns
    }
}

func WithConnMaxIdleTime(connMaxIdleTime time.Duration) Opts {
    return func(opt *Option) {
        opt.ConnMaxIdleTime = connMaxIdleTime
    }
}

func WithConnMaxLifetime(connMaxLifetime time.Duration) Opts {
    return func(opt *Option) {
        opt.ConnMaxLifetime = connMaxLifetime
    }
}

// Init 初始化数据库
func (msl *Mysql) Init(opts ...Opts) {
    mysqlOnce.Do(func() {
        str := "%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local"
        dsn := fmt.Sprintf(str, msl.Username, msl.Password, msl.Host, msl.Port, msl.Database)

        opt := Option{
            config:          gorm.Config{Logger: logger.Default.LogMode(logger.Info)},
            MaxIdleConns:    50,
            MaxOpenConns:    100,
            ConnMaxIdleTime: 30 * time.Minute,
            ConnMaxLifetime: 3 * time.Hour,
        }

        for _, o := range opts {
            o(&opt)
        }

        gdb, err := gorm.Open(mysql.Open(dsn), &opt.config)
        if err != nil {
            zlog.Fatalf("mysql gorm.Open failed: %v", err)
        }
        // 设置连接池
        db, err := gdb.DB()
        if err != nil {
            zlog.Fatalf("mysql gdb.DB failed: %v", err)
        }
        db.SetMaxIdleConns(opt.MaxIdleConns)       // 空闲
        db.SetMaxOpenConns(opt.MaxOpenConns)       // 打开
        db.SetConnMaxIdleTime(opt.ConnMaxIdleTime) // 空闲超时
        db.SetConnMaxLifetime(opt.ConnMaxLifetime) // 超时
        err = db.Ping()
        if err != nil {
            zlog.Fatalf("mysql db.Ping failed: %v", err)
        }
        dbConn = *gdb
    })
}

func GetDb() gorm.DB {
    return dbConn
}
