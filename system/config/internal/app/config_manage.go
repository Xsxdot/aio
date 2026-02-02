package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/config/internal/model"
	"xiaozhizhang/system/config/internal/model/dto"
	"xiaozhizhang/system/config/internal/service"

	"github.com/go-redis/cache/v9"
)

// CreateConfig 创建配置
func (a *App) CreateConfig(ctx context.Context, req *dto.CreateConfigRequest, operator string, operatorID int64) error {
	// 创建配置项
	configItem := &model.ConfigItem{
		Key:      req.Key,
		Value:    req.Value,
		Metadata: req.Metadata,
	}

	// 开启事务
	tx := base.DB.Begin()
	if tx.Error != nil {
		return a.err.New("开启事务失败", tx.Error).ToLog(a.log.GetLogger())
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			a.log.WithField("panic", r).Error("创建配置时发生panic")
		}
	}()

	// 使用事务创建配置
	configItemService := a.ConfigItemService.WithTx(tx)
	if err := configItemService.CreateWithEncrypt(ctx, configItem); err != nil {
		tx.Rollback()
		return err
	}

	// 查询刚创建的配置以获取ID
	createdItem, err := configItemService.FindByKey(ctx, req.Key)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 创建历史记录
	configHistoryService := a.ConfigHistoryService.WithTx(tx)
	if err := configHistoryService.CreateHistory(ctx, req.Key, 1, req.Value, req.Metadata, operator, operatorID, req.ChangeNote); err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return a.err.New("提交事务失败", err).ToLog(a.log.GetLogger())
	}

	a.log.WithField("key", req.Key).WithField("id", createdItem.ID).Info("创建配置成功")
	return nil
}

// UpdateConfig 更新配置
func (a *App) UpdateConfig(ctx context.Context, id int64, req *dto.UpdateConfigRequest, operator string, operatorID int64) error {
	// 开启事务
	tx := base.DB.Begin()
	if tx.Error != nil {
		return a.err.New("开启事务失败", tx.Error).ToLog(a.log.GetLogger())
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			a.log.WithField("panic", r).Error("更新配置时发生panic")
		}
	}()

	configItemService := a.ConfigItemService.WithTx(tx)
	configHistoryService := a.ConfigHistoryService.WithTx(tx)

	// 查询旧配置
	oldConfig, err := configItemService.FindById(ctx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 保存历史记录（保存更新前的版本）
	var oldValues map[string]*model.ConfigValue
	if err := json.Unmarshal([]byte(oldConfig.Value), &oldValues); err != nil {
		tx.Rollback()
		return errorc.New("解析旧配置值失败", err)
	}

	var oldMetadata map[string]string
	if oldConfig.Metadata != "" {
		if err := json.Unmarshal([]byte(oldConfig.Metadata), &oldMetadata); err != nil {
			tx.Rollback()
			return errorc.New("解析旧元数据失败", err)
		}
	}

	if err := configHistoryService.CreateHistory(ctx, oldConfig.Key, oldConfig.Version, oldValues, oldMetadata, operator, operatorID, req.ChangeNote); err != nil {
		tx.Rollback()
		return err
	}

	// 增加版本号
	if err := configItemService.IncrementVersion(ctx, id); err != nil {
		tx.Rollback()
		return err
	}

	// 更新配置
	newConfigItem := &model.ConfigItem{
		Value:    req.Value,
		Metadata: req.Metadata,
	}
	if err := configItemService.UpdateWithEncrypt(ctx, id, newConfigItem); err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return a.err.New("提交事务失败", err).ToLog(a.log.GetLogger())
	}

	// 清除缓存
	a.clearConfigCache(oldConfig.Key)

	a.log.WithField("key", oldConfig.Key).WithField("id", id).Info("更新配置成功")
	return nil
}

// DeleteConfig 删除配置
func (a *App) DeleteConfig(ctx context.Context, id int64) error {
	// 开启事务
	tx := base.DB.Begin()
	if tx.Error != nil {
		return a.err.New("开启事务失败", tx.Error).ToLog(a.log.GetLogger())
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			a.log.WithField("panic", r).Error("删除配置时发生panic")
		}
	}()

	configItemService := a.ConfigItemService.WithTx(tx)
	configHistoryService := a.ConfigHistoryService.WithTx(tx)

	// 查询配置
	config, err := configItemService.FindById(ctx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 删除历史记录
	if err := configHistoryService.DeleteByConfigKey(ctx, config.Key); err != nil {
		tx.Rollback()
		return err
	}

	// 删除配置
	if err := configItemService.DeleteById(ctx, id); err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return a.err.New("提交事务失败", err).ToLog(a.log.GetLogger())
	}

	// 清除缓存
	a.clearConfigCache(config.Key)

	a.log.WithField("key", config.Key).WithField("id", id).Info("删除配置成功")
	return nil
}

// GetConfigByKeyAndEnv 根据配置键和环境获取配置（带缓存）
// 返回纯对象 map（可直接用于反序列化到业务结构体）
func (a *App) GetConfigByKeyAndEnv(ctx context.Context, key string, env string) (map[string]interface{}, error) {
	// 合成完整配置键（兼容直接传入完整 key）
	fullKey, err := service.ComposeFullKey(key, env)
	if err != nil {
		return nil, err
	}

	cacheKey := fmt.Sprintf("config:%s:%s", key, env)

	var result map[string]interface{}
	err = base.Cache.Once(&cache.Item{
		Key:   cacheKey,
		Value: &result,
		TTL:   5 * time.Minute,
		Do: func(*cache.Item) (interface{}, error) {
			return a.ConfigItemService.GetConfigAsPlainObject(ctx, fullKey)
		},
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetConfigJSONByKeyAndEnv 根据配置键和环境获取配置的JSON字符串
// 返回纯对象 JSON（可直接 Unmarshal 到业务结构体）
func (a *App) GetConfigJSONByKeyAndEnv(ctx context.Context, key string, env string) (string, error) {
	plainObject, err := a.GetConfigByKeyAndEnv(ctx, key, env)
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.Marshal(plainObject)
	if err != nil {
		return "", errorc.New("序列化配置值失败", err)
	}

	return string(jsonBytes), nil
}

// RollbackConfig 回滚配置到指定版本
func (a *App) RollbackConfig(ctx context.Context, id int64, targetVersion int64, operator string, operatorID int64) error {
	// 开启事务
	tx := base.DB.Begin()
	if tx.Error != nil {
		return a.err.New("开启事务失败", tx.Error).ToLog(a.log.GetLogger())
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			a.log.WithField("panic", r).Error("回滚配置时发生panic")
		}
	}()

	configItemService := a.ConfigItemService.WithTx(tx)
	configHistoryService := a.ConfigHistoryService.WithTx(tx)

	// 查询当前配置
	currentConfig, err := configItemService.FindById(ctx, id)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 查询目标版本的历史记录
	history, err := configHistoryService.FindByConfigKeyAndVersion(ctx, currentConfig.Key, targetVersion)
	if err != nil {
		tx.Rollback()
		return err
	}

	// 解析历史版本的配置值
	var historyValues map[string]*model.ConfigValue
	if err := json.Unmarshal([]byte(history.Value), &historyValues); err != nil {
		tx.Rollback()
		return errorc.New("解析历史配置值失败", err)
	}

	var historyMetadata map[string]string
	if history.Metadata != "" {
		if err := json.Unmarshal([]byte(history.Metadata), &historyMetadata); err != nil {
			tx.Rollback()
			return errorc.New("解析历史元数据失败", err)
		}
	}

	// 保存当前版本到历史
	var currentValues map[string]*model.ConfigValue
	if err := json.Unmarshal([]byte(currentConfig.Value), &currentValues); err != nil {
		tx.Rollback()
		return errorc.New("解析当前配置值失败", err)
	}

	var currentMetadata map[string]string
	if currentConfig.Metadata != "" {
		if err := json.Unmarshal([]byte(currentConfig.Metadata), &currentMetadata); err != nil {
			tx.Rollback()
			return errorc.New("解析当前元数据失败", err)
		}
	}

	changeNote := fmt.Sprintf("回滚到版本 %d", targetVersion)
	if err := configHistoryService.CreateHistory(ctx, currentConfig.Key, currentConfig.Version, currentValues, currentMetadata, operator, operatorID, changeNote); err != nil {
		tx.Rollback()
		return err
	}

	// 增加版本号
	if err := configItemService.IncrementVersion(ctx, id); err != nil {
		tx.Rollback()
		return err
	}

	// 更新配置为历史版本的值
	newConfigItem := &model.ConfigItem{
		Value:    historyValues,
		Metadata: historyMetadata,
	}
	if err := configItemService.UpdateWithEncrypt(ctx, id, newConfigItem); err != nil {
		tx.Rollback()
		return err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return a.err.New("提交事务失败", err).ToLog(a.log.GetLogger())
	}

	// 清除缓存
	a.clearConfigCache(currentConfig.Key)

	a.log.WithField("key", currentConfig.Key).WithField("targetVersion", targetVersion).Info("回滚配置成功")
	return nil
}

// QueryConfigs 查询配置列表（分页）
func (a *App) QueryConfigs(ctx context.Context, req *dto.QueryConfigRequest) ([]*model.ConfigItemModel, int64, error) {
	page := &mvc.Page{
		PageNum: req.PageNum,
		Size:    req.Size,
	}

	if req.Key != "" {
		// 模糊查询
		return a.ConfigItemService.FindPageByKeyLike(ctx, page, req.Key)
	}

	// 查询全部
	return a.ConfigItemService.FindPage(ctx, page, nil)
}

// clearConfigCache 清除配置缓存
func (a *App) clearConfigCache(key string) {
	// 清除所有环境的缓存
	envs := []string{"dev", "test", "prod", "staging", "global"}
	for _, env := range envs {
		cacheKey := fmt.Sprintf("config:%s:%s", key, env)
		if err := base.Cache.Delete(context.Background(), cacheKey); err != nil {
			a.log.WithField("cacheKey", cacheKey).WithErr(err).Warn("清除缓存失败")
		}
	}
}
