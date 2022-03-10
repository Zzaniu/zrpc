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
	"github.com/Zzaniu/zrpc/tool/xlog"
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
)

var (
	mysqlOnce sync.Once
	gdb       *gorm.DB
)

// NewMysqlDb 初始化数据库
func NewMysqlDb(msl Mysql) *gorm.DB {
	mysqlOnce.Do(func() {
		var err error
		str := "%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local"
		dsn := fmt.Sprintf(str, msl.Username, msl.Password, msl.Host, msl.Port, msl.Database)
		gdb, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
		if err != nil {
			xlog.XLog.Fatalf("mysql gorm.Open failed: %v", err)
		}
		// 设置连接池
		db, err := gdb.DB()
		if err != nil {
			xlog.XLog.Fatalf("mysql gdb.DB failed: %v", err)
		}
		db.SetMaxIdleConns(50)                  // 空闲
		db.SetMaxOpenConns(100)                 // 打开
		db.SetConnMaxIdleTime(time.Minute * 30) // 空闲超时
		db.SetConnMaxLifetime(time.Hour * 3)    // 超时
		err = db.Ping()
		if err != nil {
			xlog.XLog.Fatalf("mysql db.Ping failed: %v", err)
		}
	})
	return gdb
}
