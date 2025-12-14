package model

// ConfigItem 配置项（用于Service层传递）
type ConfigItem struct {
	Key      string                  `json:"key"`      // 配置键
	Value    map[string]*ConfigValue `json:"value"`    // 配置值
	Version  int64                   `json:"version"`  // 版本号
	Metadata map[string]string       `json:"metadata"` // 元数据
}

// ConfigValue 配置值
type ConfigValue struct {
	Value string    `json:"value"` // 配置值
	Type  ValueType `json:"type"`  // 配置类型
}

// ValueType 配置值类型
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

// RefValue 引用值
type RefValue struct {
	Key      string `json:"key"`      // 引用的配置键
	Property string `json:"property"` // 引用的属性路径，为空表示引用整个配置项
}
