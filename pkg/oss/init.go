package oss

import (
	"context"
	"github.com/xsxdot/aio/pkg/core/config"
	"github.com/xsxdot/aio/pkg/core/logger"
)

// InitAliyunOSS 初始化阿里云OSS服务
func InitAliyunOSS(ctx context.Context, cfg *config.OssConfig) (*AliyunService, error) {

	ossProvider, err := NewAliyunService(cfg)
	if err != nil {
		return nil, err
	}

	log := logger.GetLogger().WithEntryName("AliyunOSSService")
	log.Info("阿里云OSS服务初始化完成")
	return ossProvider, nil
}
