package bootstrap

import "context"

// Component 组件接口
type Component interface {
	Name() string                                        // 组件名称
	ConfigKey() string                                   // 配置中心键名
	ConfigPtr() any                                      // 配置结构体指针（用于接收配置）
	EntityPtr() any                                      // 实体实例指针
	Start(ctx context.Context, config any) error         // 启动组件
	Stop() error                                         // 停止组件
}