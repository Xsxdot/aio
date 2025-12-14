package start

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"xiaozhizhang/pkg/core/config"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/security"

	"github.com/bsm/redislock"
	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Config struct {
	AppName      string                    `yaml:"app-name"`
	Env          string                    `yaml:"env"`
	Host         string                    `yaml:"host"`
	Port         int                       `yaml:"port"`
	Domain       string                    `yaml:"domain"`
	Jwt          config.JwtConfig          `yaml:"jwt"`
	Redis        config.RedisConfig        `yaml:"redis"`
	Database     config.Database           `yaml:"db"`
	Oss          config.OssConfig          `yaml:"oss"`
	ConfigCenter config.ConfigCenterConfig `yaml:"config"`
	Wechat       config.WechatConfig       `yaml:"wechat"`
	Proxy        config.ProxyConfig        `yaml:"proxy"`
	GRPC         config.GRPCConfig         `yaml:"grpc"`
	Systemd      config.SystemdConfig      `yaml:"systemd"`
	Nginx        config.NginxConfig        `yaml:"nginx"`
	Application  config.ApplicationConfig  `yaml:"application"`
	Server       config.ServerConfig       `yaml:"server"`
}

type Configures struct {
	Config    Config
	Logger    *logger.Log
	AdminAuth *security.AdminAuth
	UserAuth  *security.UserAuth
}

func NewConfigures(file []byte, env string) *Configures {
	var cfg Config
	err := yaml.Unmarshal(file, &cfg)
	if err != nil {
		panic(fmt.Sprintf("读取文件信息失败，因为%v", err))
	}

	cfg.Env = env
	cfg.Host, _ = getLocalIP()

	c := &Configures{
		Config: cfg,
		Logger: logger.InitLogger("debug"),
	}

	c.AdminAuth = c.EnableAdminAuth()
	c.UserAuth = c.EnableUserAuth()

	return c
}

func getPublicIP() string {
	resp, err := http.Get("https://ifconfig.me/ip")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(ip))
}

// getLocalIP 获取本机IP地址（优先获取内网IP）
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				// 优先返回内网IP
				if ipnet.IP.IsPrivate() {
					return ipnet.IP.String(), nil
				}
			}
		}
	}

	// 如果没找到内网IP，返回第一个非回环地址
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "127.0.0.1", nil
}

func (c *Configures) EnableAdminAuth() *security.AdminAuth {
	return security.NewAdminAuth([]byte(c.Config.Jwt.AdminSecret), time.Duration(c.Config.Jwt.ExpireTime)*7*24*time.Hour)
}

func (c *Configures) EnableUserAuth() *security.UserAuth {
	return security.NewUserAuth([]byte(c.Config.Jwt.Secret), time.Duration(c.Config.Jwt.ExpireTime)*14*24*time.Hour)
}

func (c *Configures) EnableRedis() *redis.Client {
	return config.InitRDB(c.Config.Redis, c.Config.Proxy)
}

func (c *Configures) EnableCache(rdb *redis.Client) *cache.Cache {
	return cache.New(&cache.Options{
		Redis:      rdb,
		LocalCache: cache.NewTinyLFU(1000, time.Minute),
	})
}

func (c *Configures) EnableLocker(rdb *redis.Client) *redislock.Client {
	return redislock.New(rdb)
}

func (c *Configures) EnablePg() *gorm.DB {
	db, err := config.InitPg(c.Config.Database, c.Config.Proxy)
	if err != nil {
		c.Logger.WithField("database", c.Config.Database.Host).WithField("err", err).Panic("failed connect database")
	}
	c.Logger.Info("connect database success")
	return db
}

func (c *Configures) EnableMysql() *gorm.DB {
	db, err := config.InitMysql(c.Config.Database, c.Config.Proxy)
	if err != nil {
		c.Logger.WithField("database", c.Config.Database.Host).WithField("err", err).Panic("failed connect database")
	}
	c.Logger.Info("connect database success")
	return db
}
