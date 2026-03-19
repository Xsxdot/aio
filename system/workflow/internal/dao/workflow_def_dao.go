package dao

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"

	"gorm.io/gorm"
)

type WorkflowDefDao struct {
	mvc.IBaseDao[model.WorkflowDefModel]
	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

func NewWorkflowDefDao(db *gorm.DB, log *logger.Log) *WorkflowDefDao {
	return &WorkflowDefDao{
		IBaseDao: mvc.NewGormDao[model.WorkflowDefModel](db),
		log:      log,
		err:      errorc.NewErrorBuilder("WorkflowDefDao"),
		db:       db,
	}
}

// FindByCode 根据 code 获取最新版本的定义
func (d *WorkflowDefDao) FindByCode(ctx context.Context, code string) (*model.WorkflowDefModel, error) {
	var item model.WorkflowDefModel
	err := mvc.ExtractDB(ctx, d.db).Where("code = ?", code).Order("version desc").First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByCodeAndVersion 根据 code 和 version 查询定义
func (d *WorkflowDefDao) FindByCodeAndVersion(ctx context.Context, code string, version int32) (*model.WorkflowDefModel, error) {
	var item model.WorkflowDefModel
	err := mvc.ExtractDB(ctx, d.db).Where("code = ? AND version = ?", code, version).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ListDefs 分页列出定义，支持 code 模糊匹配
func (d *WorkflowDefDao) ListDefs(ctx context.Context, codeLike string, pageNum, pageSize int32) ([]*model.WorkflowDefModel, int64, error) {
	if pageNum <= 0 {
		pageNum = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	db := mvc.ExtractDB(ctx, d.db).Model(&model.WorkflowDefModel{})
	if codeLike != "" {
		db = db.Where("code LIKE ?", "%"+codeLike+"%")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []*model.WorkflowDefModel
	offset := (pageNum - 1) * pageSize
	if err := db.Order("code asc, version desc").Offset(int(offset)).Limit(int(pageSize)).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// FindByIdWithTx 在事务内根据 ID 查询定义
func (d *WorkflowDefDao) FindByIdWithTx(ctx context.Context, tx *gorm.DB, id int64) (*model.WorkflowDefModel, error) {
	var item model.WorkflowDefModel
	err := tx.WithContext(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}
