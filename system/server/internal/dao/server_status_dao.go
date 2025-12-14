package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/server/internal/model"

	"gorm.io/gorm"
)

// ServerStatusDao 服务器状态数据访问层
type ServerStatusDao struct {
	mvc.IBaseDao[model.ServerStatusModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewServerStatusDao 创建服务器状态 DAO 实例
func NewServerStatusDao(db *gorm.DB, log *logger.Log) *ServerStatusDao {
	return &ServerStatusDao{
		IBaseDao: mvc.NewGormDao[model.ServerStatusModel](db),
		log:      log.WithEntryName("ServerStatusDao"),
		err:      errorc.NewErrorBuilder("ServerStatusDao"),
		db:       db,
	}
}

// FindByServerID 根据服务器 ID 查询状态
func (d *ServerStatusDao) FindByServerID(ctx context.Context, serverID int64) (*model.ServerStatusModel, error) {
	var status model.ServerStatusModel
	err := d.db.WithContext(ctx).Where("server_id = ?", serverID).First(&status).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("服务器状态不存在", err).NotFound()
		}
		return nil, d.err.New("查询服务器状态失败", err).DB()
	}
	return &status, nil
}

// UpsertByServerID 根据服务器 ID 更新或插入状态
func (d *ServerStatusDao) UpsertByServerID(ctx context.Context, status *model.ServerStatusModel) error {
	// 先查询是否存在
	var existing model.ServerStatusModel
	err := d.db.WithContext(ctx).Where("server_id = ?", status.ServerID).First(&existing).Error
	
	if err == gorm.ErrRecordNotFound {
		// 不存在，插入
		if err := d.db.WithContext(ctx).Create(status).Error; err != nil {
			return d.err.New("创建服务器状态失败", err).DB()
		}
		return nil
	}
	
	if err != nil {
		return d.err.New("查询服务器状态失败", err).DB()
	}
	
	// 存在，更新
	status.ID = existing.ID
	if err := d.db.WithContext(ctx).Model(&existing).Updates(status).Error; err != nil {
		return d.err.New("更新服务器状态失败", err).DB()
	}
	
	return nil
}

// ListByServerIDs 批量查询多个服务器的状态
func (d *ServerStatusDao) ListByServerIDs(ctx context.Context, serverIDs []int64) (map[int64]*model.ServerStatusModel, error) {
	var statusList []*model.ServerStatusModel
	err := d.db.WithContext(ctx).Where("server_id IN ?", serverIDs).Find(&statusList).Error
	if err != nil {
		return nil, d.err.New("批量查询服务器状态失败", err).DB()
	}

	// 转为 map
	statusMap := make(map[int64]*model.ServerStatusModel)
	for _, status := range statusList {
		statusMap[status.ServerID] = status
	}

	return statusMap, nil
}

