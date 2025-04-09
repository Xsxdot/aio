package config

import (
	"time"
)

// 环境相关常量
const (
	// MetadataKeyEnvironment 环境元数据键
	MetadataKeyEnvironment = "environment"

	// 预定义环境
	EnvDevelopment = "dev"
	EnvTesting     = "test"
	EnvStaging     = "stag"
	EnvProduction  = "prod"
)

type ConfigItem struct {
	Key       string                  `json:"key"`        // 配置键
	Value     map[string]*ConfigValue `json:"value"`      // 配置值
	Version   int64                   `json:"version"`    // 版本号
	UpdatedAt time.Time               `json:"updated_at"` // 更新时间
	Metadata  map[string]string       `json:"metadata"`   // 元数据
}

// EnvironmentConfig 环境配置参数
type EnvironmentConfig struct {
	Environment string   // 目标环境
	Fallbacks   []string // 环境回退顺序，如果在目标环境中找不到配置，将按此顺序查找
}

// NewEnvironmentConfig 创建环境配置参数
func NewEnvironmentConfig(env string, fallbacks ...string) *EnvironmentConfig {
	return &EnvironmentConfig{
		Environment: env,
		Fallbacks:   fallbacks,
	}
}

// DefaultEnvironmentFallbacks 获取默认的环境回退顺序
func DefaultEnvironmentFallbacks(env string) []string {
	switch env {
	case EnvProduction:
		return []string{EnvStaging}
	case EnvStaging:
		return []string{EnvTesting, EnvDevelopment}
	case EnvTesting:
		return []string{EnvDevelopment}
	default:
		return []string{}
	}
}

type ConfigValue struct {
	Value string    `json:"value"` // 配置值
	Type  ValueType `json:"type"`  // 配置类型
}

type ValueType string

const (
	ValueTypeString    ValueType = "string"
	ValueTypeInt       ValueType = "int"
	ValueTypeFloat     ValueType = "float"
	ValueTypeBool      ValueType = "bool"
	ValueTypeRef       ValueType = "ref"       // 引用其他配置项
	ValueTypeObject    ValueType = "object"    // 对象类型
	ValueTypeArray     ValueType = "array"     // 数组类型
	ValueTypeEncrypted ValueType = "encrypted" // 加密类型
)

// RefValue 表示对其他配置项的引用格式
type RefValue struct {
	Key      string `json:"key"`      // 引用的配置键
	Property string `json:"property"` // 引用的属性路径，为空表示引用整个配置项
}
