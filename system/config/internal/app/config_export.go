package app

import (
	"context"
	"encoding/json"
	"time"
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/system/config/internal/model"
	"xiaozhizhang/system/config/internal/model/dto"
)

// ExportConfigs 导出配置
func (a *App) ExportConfigs(ctx context.Context, req *dto.ExportConfigRequest) (*dto.ExportResult, error) {
	var configs []*model.ConfigItemModel
	var err error

	// 查询配置
	if len(req.Keys) > 0 {
		// 导出指定的配置
		configs, err = a.ConfigItemService.FindByKeys(ctx, req.Keys)
	} else {
		// 导出所有配置
		configs, err = a.ConfigItemService.FindAll(ctx)
	}

	if err != nil {
		return nil, err
	}

	// 转换为导出格式
	exportConfigs := make([]dto.ExportConfig, 0, len(configs))
	for _, config := range configs {
		var values map[string]*model.ConfigValue
		if err := json.Unmarshal([]byte(config.Value), &values); err != nil {
			a.log.WithField("key", config.Key).WithErr(err).Error("解析配置值失败")
			continue
		}

		var metadata map[string]string
		if config.Metadata != "" {
			if err := json.Unmarshal([]byte(config.Metadata), &metadata); err != nil {
				a.log.WithField("key", config.Key).WithErr(err).Error("解析元数据失败")
				continue
			}
		}

		// 如果指定了环境，只导出该环境的配置
		if req.Environment != "" {
			if _, ok := values[req.Environment]; ok {
				// 只保留指定环境的配置
				filteredValues := map[string]*model.ConfigValue{
					req.Environment: values[req.Environment],
				}
				values = filteredValues
			} else {
				// 跳过不包含指定环境的配置
				continue
			}
		}

		// 如果指定了目标盐值，需要重新加密
		if req.TargetSalt != "" && req.TargetSalt != base.Configures.Config.ConfigCenter.EncryptionSalt {
			for _, configValue := range values {
				if configValue.Type == model.ValueTypeEncrypted {
					// 先解密（原地修改）
					if err := a.EncryptionService.DecryptConfigValues(map[string]*model.ConfigValue{"temp": configValue}); err != nil {
						a.log.WithField("key", config.Key).WithErr(err).Error("解密配置值失败")
						return nil, err
					}
					// 使用目标盐值重新加密
					reEncrypted, err := a.EncryptionService.ReEncrypt(configValue.Value, base.Configures.Config.ConfigCenter.EncryptionSalt)
					if err != nil {
						a.log.WithField("key", config.Key).WithErr(err).Error("重新加密配置值失败")
						return nil, err
					}
					configValue.Value = reEncrypted
				}
			}
		}

		exportConfig := dto.ExportConfig{
			Key:         config.Key,
			Value:       values,
			Metadata:    metadata,
			Description: config.Description,
			Version:     config.Version,
		}
		exportConfigs = append(exportConfigs, exportConfig)
	}

	// 构建导出结果
	result := &dto.ExportResult{
		ExportTime: time.Now().Format("2006-01-02 15:04:05"),
		Salt:       maskSalt(base.Configures.Config.ConfigCenter.EncryptionSalt),
		Configs:    exportConfigs,
	}

	a.log.WithField("count", len(exportConfigs)).Info("导出配置成功")
	return result, nil
}

// ImportConfigs 导入配置
func (a *App) ImportConfigs(ctx context.Context, req *dto.ImportConfigRequest, operator string, operatorID int64) error {
	// 开启事务
	tx := base.DB.Begin()
	if tx.Error != nil {
		return a.err.New("开启事务失败", tx.Error).ToLog(a.log.GetLogger())
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			a.log.WithField("panic", r).Error("导入配置时发生panic")
		}
	}()

	configItemService := a.ConfigItemService.WithTx(tx)
	configHistoryService := a.ConfigHistoryService.WithTx(tx)

	importCount := 0
	updateCount := 0
	skipCount := 0

	for _, exportConfig := range req.Configs {
		// 如果源盐值与当前盐值不同，需要重新加密
		if req.SourceSalt != "" && req.SourceSalt != base.Configures.Config.ConfigCenter.EncryptionSalt {
			for _, configValue := range exportConfig.Value {
				if configValue.Type == model.ValueTypeEncrypted {
					reEncrypted, err := a.EncryptionService.ReEncrypt(configValue.Value, req.SourceSalt)
					if err != nil {
						tx.Rollback()
						a.log.WithField("key", exportConfig.Key).WithErr(err).Error("重新加密配置值失败")
						return err
					}
					configValue.Value = reEncrypted
				}
			}
		}

		// 检查配置是否已存在
		existingConfig, err := configItemService.FindByKey(ctx, exportConfig.Key)
		if err != nil && !errorc.IsNotFound(err) {
			tx.Rollback()
			return err
		}

		if existingConfig != nil {
			// 配置已存在
			if !req.OverWrite {
				// 不覆盖，跳过
				skipCount++
				continue
			}

			// 覆盖现有配置
			// 保存旧版本到历史
			var oldValues map[string]*model.ConfigValue
			if err := json.Unmarshal([]byte(existingConfig.Value), &oldValues); err != nil {
				tx.Rollback()
				return errorc.New("解析旧配置值失败", err)
			}

			var oldMetadata map[string]string
			if existingConfig.Metadata != "" {
				if err := json.Unmarshal([]byte(existingConfig.Metadata), &oldMetadata); err != nil {
					tx.Rollback()
					return errorc.New("解析旧元数据失败", err)
				}
			}

			if err := configHistoryService.CreateHistory(ctx, existingConfig.Key, existingConfig.Version, oldValues, oldMetadata, operator, operatorID, "导入配置覆盖"); err != nil {
				tx.Rollback()
				return err
			}

			// 增加版本号
			if err := configItemService.IncrementVersion(ctx, existingConfig.ID); err != nil {
				tx.Rollback()
				return err
			}

			// 更新配置
			newConfigItem := &model.ConfigItem{
				Value:    exportConfig.Value,
				Metadata: exportConfig.Metadata,
			}
			if err := configItemService.UpdateWithEncrypt(ctx, existingConfig.ID, newConfigItem); err != nil {
				tx.Rollback()
				return err
			}

			updateCount++

			// 清除缓存
			a.clearConfigCache(existingConfig.Key)
		} else {
			// 创建新配置
			configItem := &model.ConfigItem{
				Key:      exportConfig.Key,
				Value:    exportConfig.Value,
				Metadata: exportConfig.Metadata,
			}

			if err := configItemService.CreateWithEncrypt(ctx, configItem); err != nil {
				tx.Rollback()
				return err
			}

			// 创建历史记录
			if err := configHistoryService.CreateHistory(ctx, exportConfig.Key, 1, exportConfig.Value, exportConfig.Metadata, operator, operatorID, "导入配置"); err != nil {
				tx.Rollback()
				return err
			}

			importCount++
		}
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return a.err.New("提交事务失败", err).ToLog(a.log.GetLogger())
	}

	a.log.WithField("import", importCount).
		WithField("update", updateCount).
		WithField("skip", skipCount).
		Info("导入配置成功")

	return nil
}

// maskSalt 脱敏盐值（只显示前4位和后4位）
func maskSalt(salt string) string {
	if len(salt) <= 8 {
		return "****"
	}
	return salt[:4] + "****" + salt[len(salt)-4:]
}
