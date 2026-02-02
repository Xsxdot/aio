package app

import (
	"context"
	"xiaozhizhang/base"
	"xiaozhizhang/system/server/internal/model"
)

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
		// 兼容策略：extranetHost 优先使用 bs.ExtranetHost，否则使用 bs.Host
		extranetHost := bs.ExtranetHost
		if extranetHost == "" {
			extranetHost = bs.Host
		}

		server := &model.ServerModel{
			Name:             bs.Name,
			Host:             extranetHost,        // 兼容字段，与外网地址保持一致
			IntranetHost:     bs.IntranetHost,
			ExtranetHost:     extranetHost,
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

