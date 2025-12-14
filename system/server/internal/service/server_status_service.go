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

// ServerStatusService 服务器状态服务层
type ServerStatusService struct {
	mvc.IBaseService[model.ServerStatusModel]
	dao *dao.ServerStatusDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewServerStatusService 创建服务器状态服务实例
func NewServerStatusService(db *gorm.DB, log *logger.Log) *ServerStatusService {
	statusDao := dao.NewServerStatusDao(db, log)
	return &ServerStatusService{
		IBaseService: mvc.NewBaseService[model.ServerStatusModel](statusDao),
		dao:          statusDao,
		log:          log.WithEntryName("ServerStatusService"),
		err:          errorc.NewErrorBuilder("ServerStatusService"),
	}
}

// FindByServerID 根据服务器 ID 查询状态
func (s *ServerStatusService) FindByServerID(ctx context.Context, serverID int64) (*model.ServerStatusModel, error) {
	return s.dao.FindByServerID(ctx, serverID)
}

// ReportStatus 上报服务器状态
func (s *ServerStatusService) ReportStatus(ctx context.Context, req *dto.ReportServerStatusRequest) error {
	// 构建状态模型
	status := &model.ServerStatusModel{
		ServerID:     req.ServerID,
		CPUPercent:   req.CPUPercent,
		MemUsed:      req.MemUsed,
		MemTotal:     req.MemTotal,
		Load1:        req.Load1,
		Load5:        req.Load5,
		Load15:       req.Load15,
		CollectedAt:  req.CollectedAt,
		ErrorMessage: req.ErrorMessage,
	}

	// 处理磁盘项
	if req.DiskItems != nil {
		// 转换为内部模型
		diskItems := make([]model.DiskItem, 0, len(req.DiskItems))
		for _, item := range req.DiskItems {
			diskItems = append(diskItems, model.DiskItem{
				MountPoint: item.MountPoint,
				Used:       item.Used,
				Total:      item.Total,
				Percent:    item.Percent,
			})
		}
		status.DiskItems = diskItems
	}

	// 设置上报时间为当前时间
	status.ReportedAt = status.UpdatedAt

	// Upsert
	return s.dao.UpsertByServerID(ctx, status)
}

// ListByServerIDs 批量查询多个服务器的状态
func (s *ServerStatusService) ListByServerIDs(ctx context.Context, serverIDs []int64) (map[int64]*model.ServerStatusModel, error) {
	return s.dao.ListByServerIDs(ctx, serverIDs)
}

