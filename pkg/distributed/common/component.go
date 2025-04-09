package common

import (
	"context"

	"github.com/xsxdot/aio/app/config"
	consts "github.com/xsxdot/aio/app/const"
)

// Component 组件接口，定义所有组件必须实现的基本方法
type Component interface {
	// Start 启动组件
	Start(ctx context.Context) error
	// Stop 停止组件
	Stop(ctx context.Context) error
}

type ServerComponent interface {
	Name() string
	Status() consts.ComponentStatus
	Init(config *config.BaseConfig, body []byte) error
	Start(ctx context.Context) error
	Restart(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterMetadata() (bool, int, map[string]string)
	DefaultConfig(config *config.BaseConfig) interface{} // 返回组件的默认配置，用于前端初始化展示
	GetClientConfig() (bool, *config.ClientConfig)
}

// ComponentStatus 组件状态类型
type ComponentStatus string

const (
	// StatusCreated 已创建状态
	StatusCreated ComponentStatus = "created"
	// StatusInitialized 已初始化状态
	StatusInitialized ComponentStatus = "initialized"
	// StatusRunning 运行中状态
	StatusRunning ComponentStatus = "running"
	// StatusStopped 已停止状态
	StatusStopped ComponentStatus = "stopped"
	// StatusError 错误状态
	StatusError ComponentStatus = "error"
)

// ComponentInfo 组件信息结构
type ComponentInfo struct {
	// Name 组件名称
	Name string `json:"name"`
	// Type 组件类型
	Type string `json:"type"`
	// Status 组件状态
	Status ComponentStatus `json:"status"`
	// CreateTime 创建时间
	CreateTime string `json:"createTime"`
	// Metadata 元数据
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
