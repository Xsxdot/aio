package dto

import "xiaozhizhang/system/config/internal/model"

// ExportConfigRequest 导出配置请求
type ExportConfigRequest struct {
	Keys        []string `json:"keys" comment:"要导出的配置键列表，空表示导出全部"`
	Environment string   `json:"environment" comment:"只导出指定环境的配置"`
	TargetSalt  string   `json:"targetSalt" comment:"目标加密盐值，为空则使用当前系统盐值"`
}

// ImportConfigRequest 导入配置请求
type ImportConfigRequest struct {
	SourceSalt string         `json:"sourceSalt" comment:"源文件的加密盐值，为空则认为与当前系统相同"`
	Configs    []ExportConfig `json:"configs" validate:"required" comment:"配置列表"`
	OverWrite  bool           `json:"overWrite" comment:"是否覆盖已存在的配置"`
}

// ExportConfig 导出的配置项
type ExportConfig struct {
	Key         string                        `json:"key"`
	Value       map[string]*model.ConfigValue `json:"value"`
	Metadata    map[string]string             `json:"metadata"`
	Description string                        `json:"description"`
	Version     int64                         `json:"version"`
}

// ExportResult 导出结果
type ExportResult struct {
	ExportTime string         `json:"exportTime" comment:"导出时间"`
	Salt       string         `json:"salt" comment:"使用的加密盐值（脱敏）"`
	Configs    []ExportConfig `json:"configs" comment:"配置列表"`
}
