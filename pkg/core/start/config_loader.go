package start

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/xsxdot/aio/pkg/sdk"
	"gopkg.in/yaml.v3"
)

// LoadConfig 泛型配置加载器，支持从本地文件或配置中心加载配置
// T 必须内嵌 start.Config（通过 yaml:",inline" 标签），才能正确使用 config-center 功能
// 返回加载后的配置对象及错误（如果有）
func LoadConfig[T any](file []byte, env string) (*T, error) {
	// 1. 先解析本地文件到临时 Config，获取 ConfigSource 和 SDK 配置
	var localCfg Config
	if err := yaml.Unmarshal(file, &localCfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal local config: %w", err)
	}

	var result T

	// 2. 根据 ConfigSource 决定加载方式
	if localCfg.ConfigSource == "config-center" {
		// 从配置中心加载
		loaded, err := loadConfigFromCenterGeneric[T](localCfg, env)
		if err != nil {
			return nil, err
		}
		result = *loaded
	} else {
		// 从本地文件加载
		if err := yaml.Unmarshal(file, &result); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config: %w", err)
		}
	}

	// 3. 通过反射把 Env/Host 等字段补齐到内嵌的 Config
	if err := enrichConfigFields(&result, env, localCfg); err != nil {
		return nil, fmt.Errorf("failed to enrich config fields: %w", err)
	}

	return &result, nil
}

// MustLoadConfig 与 LoadConfig 相同，但遇到错误会 panic（快速启动风格）
func MustLoadConfig[T any](file []byte, env string) *T {
	cfg, err := LoadConfig[T](file, env)
	if err != nil {
		panic(fmt.Sprintf("加载配置失败，因为%v", err))
	}
	return cfg
}

// loadConfigFromCenterGeneric 从配置中心加载配置的泛型版本
func loadConfigFromCenterGeneric[T any](localCfg Config, env string) (*T, error) {
	// 验证必要的 SDK 配置
	if localCfg.Sdk.RegistryAddr == "" {
		return nil, fmt.Errorf("sdk.registry_addr is required for config-center mode")
	}
	if localCfg.Sdk.ClientKey == "" {
		return nil, fmt.Errorf("sdk.client_key is required for config-center mode")
	}
	if localCfg.Sdk.ClientSecret == "" {
		return nil, fmt.Errorf("sdk.client_secret is required for config-center mode")
	}
	if localCfg.Sdk.BootstrapConfigPrefix == "" {
		return nil, fmt.Errorf("sdk.bootstrap_config_prefix is required for config-center mode")
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
		return nil, fmt.Errorf("failed to create sdk client: %w", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	prefix := localCfg.Sdk.BootstrapConfigPrefix

	// 先尝试直接用 prefix 作为 key 获取完整配置
	configJSON, err := client.ConfigClient.GetConfigJSON(ctx, prefix)
	if err != nil && !sdk.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get config from center: %w", err)
	}

	// 如果找不到完整配置，则按前缀查询并组装
	if sdk.IsNotFound(err) || configJSON == "" {
		configJSON, err = loadAndComposeConfigsByPrefix(ctx, client, prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to compose configs by prefix: %w", err)
		}
	}

	// 反序列化为 T（yaml.Unmarshal 支持 JSON 输入）
	var cfg T
	if err := yaml.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config from center: %w", err)
	}

	return &cfg, nil
}

// enrichConfigFields 通过反射将运行时信息（Env/Host/Sdk/ConfigSource）填充到内嵌的 Config
func enrichConfigFields(target interface{}, env string, localCfg Config) error {
	val := reflect.ValueOf(target)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a struct or pointer to struct")
	}

	// 查找内嵌的 start.Config 字段（通常是匿名字段或名为 Config 的字段）
	var configField reflect.Value
	typ := val.Type()

	// 优先查找匿名字段（inline）
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// 检查是否是 start.Config 类型的匿名字段
		if fieldType.Anonymous && fieldType.Type.Name() == "Config" {
			configField = field
			break
		}

		// 也支持显式命名的 Config 字段
		if fieldType.Name == "Config" && fieldType.Type.Name() == "Config" {
			configField = field
			break
		}
	}

	if !configField.IsValid() {
		// 如果找不到内嵌的 Config，尝试直接赋值（target 本身就是 Config）
		if typ.Name() == "Config" {
			configField = val
		} else {
			return fmt.Errorf("config field not found in target struct (expected embedded start.Config)")
		}
	}

	// 设置字段值
	if configField.CanSet() {
		// Env
		envField := configField.FieldByName("Env")
		if envField.IsValid() && envField.CanSet() {
			envField.SetString(env)
		}

		// Host
		hostField := configField.FieldByName("Host")
		if hostField.IsValid() && hostField.CanSet() {
			host, _ := getLocalIP()
			hostField.SetString(host)
		}

		// Sdk（保留本地配置）
		sdkField := configField.FieldByName("Sdk")
		if sdkField.IsValid() && sdkField.CanSet() {
			sdkField.Set(reflect.ValueOf(localCfg.Sdk))
		}

		// ConfigSource（保留本地配置）
		sourceField := configField.FieldByName("ConfigSource")
		if sourceField.IsValid() && sourceField.CanSet() {
			sourceField.SetString(localCfg.ConfigSource)
		}
	}

	return nil
}
