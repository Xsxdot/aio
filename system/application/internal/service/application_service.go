package service

import (
	"context"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/application/internal/dao"
	"xiaozhizhang/system/application/internal/model"
)

// ApplicationService 应用服务
type ApplicationService struct {
	dao *dao.ApplicationDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewApplicationService 创建应用服务实例
func NewApplicationService(dao *dao.ApplicationDao, log *logger.Log) *ApplicationService {
	return &ApplicationService{
		dao: dao,
		log: log.WithEntryName("ApplicationService"),
		err: errorc.NewErrorBuilder("ApplicationService"),
	}
}

// Create 创建应用
func (s *ApplicationService) Create(ctx context.Context, app *model.Application) error {
	// 检查是否已存在
	exists, err := s.dao.ExistsByKey(ctx, app.Project, app.Name, app.Env)
	if err != nil {
		return err
	}
	if exists {
		return s.err.New("应用已存在", nil).ValidWithCtx()
	}

	// 设置默认值
	if app.Status == 0 {
		app.Status = 1
	}

	return s.dao.Create(ctx, app)
}

// FindByID 根据 ID 查找应用
func (s *ApplicationService) FindByID(ctx context.Context, id int64) (*model.Application, error) {
	return s.dao.FindById(ctx, id)
}

// FindByKey 根据唯一键查找应用
func (s *ApplicationService) FindByKey(ctx context.Context, project, name, env string) (*model.Application, error) {
	return s.dao.FindByKey(ctx, project, name, env)
}

// ListByFilter 根据条件查询应用列表
func (s *ApplicationService) ListByFilter(ctx context.Context, project, env, appType, keyword string) ([]*model.Application, error) {
	return s.dao.ListByFilter(ctx, project, env, appType, keyword)
}

// Update 更新应用
func (s *ApplicationService) Update(ctx context.Context, id int64, updates *model.Application) (*model.Application, error) {
	_, err := s.dao.UpdateById(ctx, id, updates)
	if err != nil {
		return nil, err
	}
	return s.dao.FindById(ctx, id)
}

// Delete 删除应用
func (s *ApplicationService) Delete(ctx context.Context, id int64) error {
	return s.dao.DeleteById(ctx, id)
}

// UpdateCurrentRelease 更新当前运行的 Release ID
func (s *ApplicationService) UpdateCurrentRelease(ctx context.Context, id int64, releaseID int64) error {
	_, err := s.dao.UpdateById(ctx, id, &model.Application{CurrentReleaseID: releaseID})
	return err
}

