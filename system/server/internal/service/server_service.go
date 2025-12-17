package service

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/server/internal/dao"
	"xiaozhizhang/system/server/internal/model"
	"xiaozhizhang/system/server/internal/model/dto"

	"gorm.io/gorm"
)

// ServerService 服务器服务层
type ServerService struct {
	mvc.IBaseService[model.ServerModel]
	dao *dao.ServerDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewServerService 创建服务器服务实例
func NewServerService(db *gorm.DB, log *logger.Log) *ServerService {
	serverDao := dao.NewServerDao(db, log)
	return &ServerService{
		IBaseService: mvc.NewBaseService[model.ServerModel](serverDao),
		dao:          serverDao,
		log:          log.WithEntryName("ServerService"),
		err:          errorc.NewErrorBuilder("ServerService"),
	}
}

// Create 创建服务器
func (s *ServerService) Create(ctx context.Context, req *dto.CreateServerRequest) (*model.ServerModel, error) {
	// 检查名称是否已存在
	existing, err := s.dao.FindByName(ctx, req.Name)
	if err != nil && !errorc.IsNotFound(err) {
		return nil, err
	}
	if existing != nil {
		return nil, s.err.New("服务器名称已存在", nil).ValidWithCtx()
	}

	// 构建模型
	server := &model.ServerModel{
		Name:             req.Name,
		Host:             req.Host,
		AgentGrpcAddress: req.AgentGrpcAddress,
		Enabled:          req.Enabled,
		Comment:          req.Comment,
	}

	// 处理标签
	if req.Tags != nil {
		tags := make(map[string]interface{})
		for k, v := range req.Tags {
			tags[k] = v
		}
		server.Tags = tags
	}

	// 创建
	if err := s.dao.Create(ctx, server); err != nil {
		return nil, err
	}

	return server, nil
}

// Update 更新服务器
func (s *ServerService) Update(ctx context.Context, id int64, req *dto.UpdateServerRequest) error {
	// 查询服务器
	server, err := s.dao.FindById(ctx, id)
	if err != nil {
		return err
	}

	// 如果更新名称，检查是否重复
	if req.Name != nil && *req.Name != server.Name {
		existing, err := s.dao.FindByName(ctx, *req.Name)
		if err != nil && !errorc.IsNotFound(err) {
			return err
		}
		if existing != nil && existing.ID != id {
			return s.err.New("服务器名称已存在", nil).ValidWithCtx()
		}
		server.Name = *req.Name
	}

	// 更新字段
	if req.Host != nil {
		server.Host = *req.Host
	}
	if req.AgentGrpcAddress != nil {
		server.AgentGrpcAddress = *req.AgentGrpcAddress
	}
	if req.Enabled != nil {
		server.Enabled = *req.Enabled
	}
	if req.Comment != nil {
		server.Comment = *req.Comment
	}
	if req.Tags != nil {
		tags := make(map[string]interface{})
		for k, v := range req.Tags {
			tags[k] = v
		}
		server.Tags = tags
	}

	// 更新
	_, err = s.dao.UpdateById(ctx, id, server)
	return err
}

// QueryWithPage 分页查询服务器
func (s *ServerService) QueryWithPage(ctx context.Context, req *dto.QueryServerRequest) ([]*model.ServerModel, int64, error) {
	return s.dao.QueryWithPage(ctx, req.Name, req.Tag, req.Enabled, req.PageNum, req.Size)
}

// ListByEnabled 根据启用状态查询服务器列表
func (s *ServerService) ListByEnabled(ctx context.Context, enabled bool) ([]*model.ServerModel, error) {
	return s.dao.ListByEnabled(ctx, enabled)
}

// ListAll 查询所有服务器
func (s *ServerService) ListAll(ctx context.Context) ([]*model.ServerModel, error) {
	return s.dao.ListAll(ctx)
}

// UpsertByName 根据名称更新或插入服务器（用于 bootstrap）
func (s *ServerService) UpsertByName(ctx context.Context, server *model.ServerModel) error {
	return s.dao.UpsertByName(ctx, server)
}

// FindByName 根据名称查询服务器（用于 bootstrap 初始化）
func (s *ServerService) FindByName(ctx context.Context, name string) (*model.ServerModel, error) {
	return s.dao.FindByName(ctx, name)
}
