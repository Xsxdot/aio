package app

import (
	"context"
	"xiaozhizhang/base"
	"xiaozhizhang/system/server/internal/model"
)

// BootstrapServer bootstrap 服务器配置项
type BootstrapServer struct {
	Name             string            `yaml:"name"`
	Host             string            `yaml:"host"`
	AgentGrpcAddress string            `yaml:"agent_grpc_address"`
	Enabled          bool              `yaml:"enabled"`
	Tags             map[string]string `yaml:"tags"`
	Comment          string            `yaml:"comment"`
}

// EnsureBootstrapServers 确保 bootstrap 服务器已存在（启动时调用）
func (a *App) EnsureBootstrapServers(ctx context.Context) error {
	// 从配置中读取 bootstrap 服务器列表
	bootstrapServers := base.Configures.Config.Server.Bootstrap
	if len(bootstrapServers) == 0 {
		a.log.Info("没有配置 bootstrap 服务器，跳过初始化")
		return nil
	}

	a.log.WithField("count", len(bootstrapServers)).Info("开始初始化 bootstrap 服务器")

	// 逐个 upsert
	for _, bs := range bootstrapServers {
		server := &model.ServerModel{
			Name:             bs.Name,
			Host:             bs.Host,
			AgentGrpcAddress: bs.AgentGrpcAddress,
			Enabled:          bs.Enabled,
			Comment:          bs.Comment,
		}

		// 处理标签
		if bs.Tags != nil {
			tags := make(map[string]interface{})
			for k, v := range bs.Tags {
				tags[k] = v
			}
			server.Tags = tags
		}

		// Upsert
		if err := a.ServerService.UpsertByName(ctx, server); err != nil {
			a.log.WithErr(err).WithField("name", bs.Name).Error("初始化 bootstrap 服务器失败")
			return err
		}

		a.log.WithField("name", bs.Name).Info("bootstrap 服务器初始化成功")
	}

	a.log.Info("bootstrap 服务器初始化完成")
	return nil
}

