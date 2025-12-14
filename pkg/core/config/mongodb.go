package config

import (
	"errors"
	"net"

	"gopkg.in/mgo.v2"
)

type MongoConfig struct {
	Host            string `yaml:"host"`
	DBName          string `yaml:"db-name"`
	UserName        string `yaml:"user-name"`
	Password        string `yaml:"password"`
	ConsistencyMode string `yaml:"consistency-mode"`
}

func InitMongo(config MongoConfig, proxyConfig ProxyConfig) (*mgo.Database, error) {
	var dial *mgo.Session
	var err error

	if proxyConfig.Enabled {
		// 使用代理连接MongoDB
		dialer := proxyConfig.GetDialer()

		dialInfo := &mgo.DialInfo{
			Addrs:    []string{config.Host},
			Database: config.DBName,
			Username: config.UserName,
			Password: config.Password,
			DialServer: func(addr *mgo.ServerAddr) (net.Conn, error) {
				return dialer.Dial("tcp", addr.String())
			},
		}

		dial, err = mgo.DialWithInfo(dialInfo)
		if err != nil {
			return nil, err
		}
	} else {
		// 不使用代理的常规连接
		dial, err = mgo.Dial(config.Host)
		if err != nil {
			return nil, err
		}
	}

	if config.DBName == "" {
		return nil, errors.New("数据库名不存在")
	}

	switch config.ConsistencyMode {
	case "monotonic":
		dial.SetMode(mgo.Monotonic, false)
	case "eventual":
		dial.SetMode(mgo.Eventual, true)
	default:
		dial.SetMode(mgo.Strong, false)
	}

	db := dial.DB(config.DBName)

	// 如果使用代理连接，登录逻辑已在DialInfo中处理
	// 否则需要单独登录
	if !proxyConfig.Enabled && config.UserName != "" && config.Password != "" {
		err := db.Login(config.UserName, config.Password)
		if err != nil {
			return nil, err
		}
	}

	dial.SetPoolLimit(100)

	return db, nil
}
