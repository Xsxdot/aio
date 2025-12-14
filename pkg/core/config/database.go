package config

import (
	"context"
	"fmt"
	"net"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Database struct {
	Host     string `yaml:"host" json:"host,omitempty"`
	Port     int64  `yaml:"port" json:"port,omitempty"`
	User     string `yaml:"user" json:"user,omitempty"`
	Password string `yaml:"password" json:"password,omitempty"`
	DbName   string `yaml:"db-name" json:"db-name,omitempty"`
}

func InitPg(database Database, proxyConfig ProxyConfig) (*gorm.DB, error) {
	// 构建DSN
	dsn := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=disable password=%s",
		database.Host, database.Port, database.User, database.DbName, database.Password)

	// 打开连接
	config := &gorm.Config{}
	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, err
	}

	// 获取底层的sql.DB并配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 注意：PostgreSQL的驱动不直接支持自定义dialer
	// 代理功能需要在网络层面配置或使用支持代理的pgx驱动
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db, nil
}

func InitMysql(database Database, proxyConfig ProxyConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector

	if proxyConfig.Enabled {
		// 注册自定义dialer到MySQL驱动
		dialerName := fmt.Sprintf("proxy_%d", time.Now().UnixNano())
		dialer := proxyConfig.GetDialer()

		mysqldriver.RegisterDialContext(dialerName, func(ctx context.Context, addr string) (net.Conn, error) {
			return dialer.Dial("tcp", addr)
		})

		// 构建DSN，使用自定义dialer
		dsn := fmt.Sprintf("%s:%s@%s(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			database.User, database.Password, dialerName, database.Host, database.Port, database.DbName)
		dialector = mysql.Open(dsn)
	} else {
		// 不使用代理的常规连接
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			database.User, database.Password, database.Host, database.Port, database.DbName)
		dialector = mysql.Open(dsn)
	}

	return gorm.Open(dialector, &gorm.Config{})
}
