package start

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/xsxdot/aio/pkg/core/config"
	"github.com/xsxdot/aio/pkg/sdk"
	config2 "github.com/xsxdot/gokit/config"
	"github.com/xsxdot/gokit/logger"
	"github.com/xsxdot/gokit/security"

	"github.com/bsm/redislock"
	"github.com/go-redis/cache/v9"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Config struct {
	ConfigSource string                    `yaml:"config-source"` // 配置来源：file 或 config-center
	AppName      string                    `yaml:"app-name"`
	Env          string                    `yaml:"env"`
	Host         string                    `yaml:"host"`
	Port         int                       `yaml:"port"`
	Domain       string                    `yaml:"domain"`
	LogLevel     string                    `yaml:"log-level"` // 日志级别：debug/info/warn
	Jwt          config.JwtConfig          `yaml:"jwt"`
	Redis        config.RedisConfig        `yaml:"redis"`
	Database     config.Database           `yaml:"db"`
	Oss          config2.OssConfig         `yaml:"oss"`
	ConfigCenter config.ConfigCenterConfig `yaml:"config"`
	Wechat       config.WechatConfig       `yaml:"wechat"`
	Proxy        config.ProxyConfig        `yaml:"proxy"`
	GRPC         config.GRPCConfig         `yaml:"grpc"`
	Sdk          config.SdkConfig          `yaml:"sdk"`
}

type Configures struct {
	Config    Config
	Logger    *logger.Log
	AdminAuth *security.AdminAuth
	UserAuth  *security.UserAuth
}

// NewConfigures 从文件加载配置并创建 Configures 实例（兼容旧 API）
// 内部使用 LoadConfig[Config] + NewConfiguresFromConfig 实现
func NewConfigures(file []byte, env string) *Configures {
	cfg := MustLoadConfig[Config](file, env)
	return NewConfiguresFromConfig(*cfg)
}

// NewConfiguresFromConfig 从已加载的 Config 创建 Configures 实例
// 推荐其他项目使用此方法，配合泛型 LoadConfig[T] 加载自定义配置
func NewConfiguresFromConfig(cfg Config) *Configures {
	// 日志级别：配置为空则默认 info
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}

	c := &Configures{
		Config: cfg,
		Logger: logger.InitLogger(logLevel),
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

// EnableDB 根据配置的 db.type 返回 MySQL 或 PostgreSQL 连接，默认 mysql
func (c *Configures) EnableDB() *gorm.DB {
	dbType := c.Config.Database.Type
	if dbType == "" {
		dbType = "mysql"
	}
	switch dbType {
	case "postgres":
		return c.EnablePg()
	case "mysql":
		fallthrough
	default:
		return c.EnableMysql()
	}
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

// EnableSDK 创建并返回 SDK 客户端
func (c *Configures) EnableSDK() *sdk.Client {
	if c.Config.Sdk.RegistryAddr == "" {
		c.Logger.Panic("sdk.registry_addr is required")
	}
	if c.Config.Sdk.ClientKey == "" {
		c.Logger.Panic("sdk.client_key is required")
	}
	if c.Config.Sdk.ClientSecret == "" {
		c.Logger.Panic("sdk.client_secret is required")
	}

	sdkConfig := sdk.Config{
		RegistryAddr:   c.Config.Sdk.RegistryAddr,
		ClientKey:      c.Config.Sdk.ClientKey,
		ClientSecret:   c.Config.Sdk.ClientSecret,
		DefaultTimeout: c.Config.Sdk.DefaultTimeout,
		DisableAuth:    c.Config.Sdk.DisableAuth,
		Env:            c.Config.Env,
	}

	client, err := sdk.New(sdkConfig)
	if err != nil {
		c.Logger.WithField("registry_addr", c.Config.Sdk.RegistryAddr).WithField("err", err).Panic("failed create sdk client")
	}

	c.Logger.WithField("registry_addr", c.Config.Sdk.RegistryAddr).Info("sdk client created successfully")
	return client
}

// EnableSDKAndRegisterSelf 创建 SDK 客户端并自动注册到注册中心（带心跳）
// 使用 EnsureService + RegisterInstance 的完整注册流程，不需要预先创建服务
// 返回 (client, handle)，调用方需在程序退出时调用 handle.Stop() 注销实例
// loadConfigFromCenter 从配置中心加载配置
func loadConfigFromCenter(localCfg Config, env string) (Config, error) {
	// 验证必要的 SDK 配置
	if localCfg.Sdk.RegistryAddr == "" {
		return Config{}, fmt.Errorf("sdk.registry_addr is required for config-center mode")
	}
	if localCfg.Sdk.ClientKey == "" {
		return Config{}, fmt.Errorf("sdk.client_key is required for config-center mode")
	}
	if localCfg.Sdk.ClientSecret == "" {
		return Config{}, fmt.Errorf("sdk.client_secret is required for config-center mode")
	}
	if localCfg.Sdk.BootstrapConfigPrefix == "" {
		return Config{}, fmt.Errorf("sdk.bootstrap_config_prefix is required for config-center mode")
	}

	// 创建 SDK 客户端
	sdkConfig := sdk.Config{
		RegistryAddr:   localCfg.Sdk.RegistryAddr,
		ClientKey:      localCfg.Sdk.ClientKey,
		ClientSecret:   localCfg.Sdk.ClientSecret,
		DefaultTimeout: localCfg.Sdk.DefaultTimeout,
		DisableAuth:    localCfg.Sdk.DisableAuth,
		Env:            env,
	}
	if sdkConfig.DefaultTimeout == 0 {
		sdkConfig.DefaultTimeout = 30 * time.Second
	}

	client, err := sdk.New(sdkConfig)
	if err != nil {
		return Config{}, fmt.Errorf("failed to create sdk client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefix := localCfg.Sdk.BootstrapConfigPrefix

	// 先尝试直接用 prefix 作为 key 获取完整配置
	configJSON, err := client.ConfigClient.GetConfigJSON(ctx, prefix)
	if err != nil && !sdk.IsNotFound(err) {
		return Config{}, fmt.Errorf("failed to get config from center: %w", err)
	}

	// 如果找不到完整配置，则按前缀查询并组装
	if sdk.IsNotFound(err) || configJSON == "" {
		configJSON, err = client.ConfigClient.GetComposedConfigByPrefix(ctx, prefix)
		if err != nil {
			return Config{}, fmt.Errorf("failed to compose configs by prefix: %w", err)
		}
	}

	// 反序列化为 Config（yaml.Unmarshal 支持 JSON 输入）
	var cfg Config
	if err := yaml.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config from center: %w", err)
	}

	// 保留本地的 Sdk 配置和 ConfigSource
	cfg.Sdk = localCfg.Sdk
	cfg.ConfigSource = localCfg.ConfigSource

	return cfg, nil
}

func (c *Configures) EnableSDKAndRegisterSelf() (*sdk.Client, *sdk.RegistrationHandle) {
	// 先创建 SDK 客户端
	client := c.EnableSDK()

	// 检查必填字段
	if c.Config.Sdk.Register.Project == "" {
		c.Logger.Panic("sdk.register.project is required for auto registration")
	}
	if c.Config.Sdk.Register.Name == "" {
		c.Logger.Panic("sdk.register.name is required for auto registration")
	}
	if c.Config.Sdk.Register.Owner == "" {
		c.Logger.Panic("sdk.register.owner is required for auto registration")
	}

	// 准备服务确保请求（EnsureService）
	svcReq := &sdk.EnsureServiceRequest{
		Project:     c.Config.Sdk.Register.Project,
		Name:        c.Config.Sdk.Register.Name,
		Owner:       c.Config.Sdk.Register.Owner,
		Description: c.Config.Sdk.Register.Description,
		SpecJSON:    c.Config.Sdk.Register.SpecJSON,
	}

	// Description: 为空则使用默认值
	if svcReq.Description == "" {
		svcReq.Description = fmt.Sprintf("%s service", c.Config.AppName)
	}

	// SpecJSON: 为空则使用默认值
	if svcReq.SpecJSON == "" {
		svcReq.SpecJSON = "{}"
	}

	// 准备实例注册请求（RegisterInstance）
	instReq := &sdk.RegisterInstanceRequest{}

	// InstanceKey: 为空则自动生成
	if c.Config.Sdk.Register.InstanceKey != "" {
		instReq.InstanceKey = c.Config.Sdk.Register.InstanceKey
	} else {
		instReq.InstanceKey = fmt.Sprintf("%s-%s-%d", c.Config.AppName, c.Config.Host, time.Now().Unix())
	}

	// Env: 为空则用全局 env
	if c.Config.Sdk.Register.Env != "" {
		instReq.Env = c.Config.Sdk.Register.Env
	} else {
		instReq.Env = c.Config.Env
	}

	// Host: 为空则用全局 host
	if c.Config.Sdk.Register.Host != "" {
		instReq.Host = c.Config.Sdk.Register.Host
	} else {
		instReq.Host = c.Config.Host
	}

	// Endpoint: 为空则自动生成
	if c.Config.Sdk.Register.Endpoint != "" {
		instReq.Endpoint = c.Config.Sdk.Register.Endpoint
	} else {
		instReq.Endpoint = fmt.Sprintf("http://%s:%d", c.Config.Host, c.Config.Port)
	}

	// MetaJSON: 使用配置中的值，默认为空字符串
	instReq.MetaJSON = c.Config.Sdk.Register.MetaJSON
	if instReq.MetaJSON == "" {
		instReq.MetaJSON = "{}"
	}

	// TTLSeconds: 为 0 则用默认值 60
	if c.Config.Sdk.Register.TTLSeconds > 0 {
		instReq.TTLSeconds = c.Config.Sdk.Register.TTLSeconds
	} else {
		instReq.TTLSeconds = 60
	}

	// 注册到注册中心（使用 EnsureService + RegisterInstance 完整流程）
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	handle, err := client.Registry.RegisterSelfWithEnsureService(ctx, svcReq, instReq)
	if err != nil {
		c.Logger.WithField("project", svcReq.Project).
			WithField("service_name", svcReq.Name).
			WithField("instance_key", instReq.InstanceKey).
			WithField("err", err).
			Panic("failed to register self to registry")
	}

	c.Logger.WithField("project", svcReq.Project).
		WithField("service_name", svcReq.Name).
		WithField("instance_key", instReq.InstanceKey).
		WithField("endpoint", instReq.Endpoint).
		Info("successfully registered to registry, heartbeat started")

	return client, handle
}
