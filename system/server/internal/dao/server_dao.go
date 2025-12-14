package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/server/internal/model"

	"gorm.io/gorm"
)

// ServerDao 服务器数据访问层
type ServerDao struct {
	mvc.IBaseDao[model.ServerModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewServerDao 创建服务器 DAO 实例
func NewServerDao(db *gorm.DB, log *logger.Log) *ServerDao {
	return &ServerDao{
		IBaseDao: mvc.NewGormDao[model.ServerModel](db),
		log:      log.WithEntryName("ServerDao"),
		err:      errorc.NewErrorBuilder("ServerDao"),
		db:       db,
	}
}

// FindByName 根据名称查询服务器
func (d *ServerDao) FindByName(ctx context.Context, name string) (*model.ServerModel, error) {
	var server model.ServerModel
	err := d.db.WithContext(ctx).Where("name = ?", name).First(&server).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, d.err.New("服务器不存在", err).NotFound()
		}
		return nil, d.err.New("查询服务器失败", err).DB()
	}
	return &server, nil
}

// ListByEnabled 根据启用状态查询服务器列表
func (d *ServerDao) ListByEnabled(ctx context.Context, enabled bool) ([]*model.ServerModel, error) {
	var servers []*model.ServerModel
	err := d.db.WithContext(ctx).Where("enabled = ?", enabled).Find(&servers).Error
	if err != nil {
		return nil, d.err.New("查询服务器列表失败", err).DB()
	}
	return servers, nil
}

// ListAll 查询所有服务器
func (d *ServerDao) ListAll(ctx context.Context) ([]*model.ServerModel, error) {
	var servers []*model.ServerModel
	err := d.db.WithContext(ctx).Find(&servers).Error
	if err != nil {
		return nil, d.err.New("查询服务器列表失败", err).DB()
	}
	return servers, nil
}

// QueryWithPage 分页查询服务器
func (d *ServerDao) QueryWithPage(ctx context.Context, name string, tag string, enabled *bool, pageNum, size int) ([]*model.ServerModel, int64, error) {
	query := d.db.WithContext(ctx).Model(&model.ServerModel{})

	// 名称过滤
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// 标签过滤
	if tag != "" {
		query = query.Where("JSON_CONTAINS(tags, ?)", "\""+tag+"\"")
	}

	// 启用状态过滤
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, d.err.New("统计服务器数量失败", err).DB()
	}

	// 分页查询
	var servers []*model.ServerModel
	offset := (pageNum - 1) * size
	err := query.Offset(offset).Limit(size).Order("id DESC").Find(&servers).Error
	if err != nil {
		return nil, 0, d.err.New("分页查询服务器失败", err).DB()
	}

	return servers, total, nil
}

// UpsertByName 根据名称更新或插入服务器
func (d *ServerDao) UpsertByName(ctx context.Context, server *model.ServerModel) error {
	// 先查询是否存在
	var existing model.ServerModel
	err := d.db.WithContext(ctx).Where("name = ?", server.Name).First(&existing).Error
	
	if err == gorm.ErrRecordNotFound {
		// 不存在，插入
		if err := d.db.WithContext(ctx).Create(server).Error; err != nil {
			return d.err.New("创建服务器失败", err).DB()
		}
		return nil
	}
	
	if err != nil {
		return d.err.New("查询服务器失败", err).DB()
	}
	
	// 存在，更新
	server.ID = existing.ID
	if err := d.db.WithContext(ctx).Model(&existing).Updates(server).Error; err != nil {
		return d.err.New("更新服务器失败", err).DB()
	}
	
	return nil
}

