package grpc

import (
	"context"
	"time"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/server/api/client"
	pb "xiaozhizhang/system/server/api/proto"
	internalapp "xiaozhizhang/system/server/internal/app"
	"xiaozhizhang/system/server/internal/model/dto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ServerService gRPC 服务器服务实现
type ServerService struct {
	pb.UnimplementedServerServiceServer
	client *client.ServerClient
	app    *internalapp.App
	err    *errorc.ErrorBuilder
	log    *logger.Log
}

// NewServerService 创建服务器服务实例
func NewServerService(client *client.ServerClient, app *internalapp.App, log *logger.Log) *ServerService {
	return &ServerService{
		client: client,
		app:    app,
		err:    errorc.NewErrorBuilder("ServerGRPCService"),
		log:    log,
	}
}

// ServiceName 返回服务名称
func (s *ServerService) ServiceName() string {
	return "server.v1.ServerService"
}

// ServiceVersion 返回服务版本
func (s *ServerService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册服务到 gRPC 服务器
func (s *ServerService) RegisterService(server *grpc.Server) error {
	pb.RegisterServerServiceServer(server, s)
	return nil
}

// ReportServerStatus 上报服务器状态
func (s *ServerService) ReportServerStatus(ctx context.Context, req *pb.ReportServerStatusRequest) (*pb.ReportServerStatusResponse, error) {
	// 转换为内部 DTO
	reportReq := &dto.ReportServerStatusRequest{
		ServerID:     req.ServerId,
		CPUPercent:   req.Status.CpuPercent,
		MemUsed:      req.Status.MemUsed,
		MemTotal:     req.Status.MemTotal,
		Load1:        req.Status.Load1,
		Load5:        req.Status.Load5,
		Load15:       req.Status.Load15,
		CollectedAt:  time.Unix(req.Status.CollectedAt, 0),
		ErrorMessage: req.Status.ErrorMessage,
	}

	// 转换磁盘项
	if req.Status.DiskItems != nil {
		reportReq.DiskItems = make([]dto.DiskItemDTO, 0, len(req.Status.DiskItems))
		for _, item := range req.Status.DiskItems {
			reportReq.DiskItems = append(reportReq.DiskItems, dto.DiskItemDTO{
				MountPoint: item.MountPoint,
				Used:       item.Used,
				Total:      item.Total,
				Percent:    item.Percent,
			})
		}
	}

	// 调用 app 层
	if err := s.app.ReportServerStatus(ctx, reportReq); err != nil {
		s.log.WithErr(err).WithField("server_id", req.ServerId).Error("上报服务器状态失败")
		return nil, convertToGRPCError(err)
	}

	return &pb.ReportServerStatusResponse{
		Success: true,
		Message: "上报成功",
	}, nil
}

// GetAllServerStatus 获取所有服务器状态
func (s *ServerService) GetAllServerStatus(ctx context.Context, req *pb.GetAllServerStatusRequest) (*pb.GetAllServerStatusResponse, error) {
	// 调用 client
	servers, err := s.client.GetAllServerStatus(ctx)
	if err != nil {
		s.log.WithErr(err).Error("获取所有服务器状态失败")
		return nil, convertToGRPCError(err)
	}

	// 转换为 proto 响应
	content := make([]*pb.ServerStatusInfo, 0, len(servers))
	for _, server := range servers {
		pbServer := convertToProtoServerStatusInfo(server)
		content = append(content, pbServer)
	}

	return &pb.GetAllServerStatusResponse{
		Servers: content,
	}, nil
}

// GetServerStatus 获取单个服务器状态
func (s *ServerService) GetServerStatus(ctx context.Context, req *pb.GetServerStatusRequest) (*pb.ServerStatusInfo, error) {
	// 调用 client
	server, err := s.client.GetServerStatusByID(ctx, req.ServerId)
	if err != nil {
		s.log.WithErr(err).WithField("server_id", req.ServerId).Error("获取服务器状态失败")
		return nil, convertToGRPCError(err)
	}

	return convertToProtoServerStatusInfo(server), nil
}

// convertToProtoServerStatusInfo 转换为 proto 格式
func convertToProtoServerStatusInfo(server *dto.ServerStatusInfo) *pb.ServerStatusInfo {
	pbServer := &pb.ServerStatusInfo{
		Id:               server.ID,
		Name:             server.Name,
		Host:             server.Host,
		AgentGrpcAddress: server.AgentGrpcAddress,
		Enabled:          server.Enabled,
		Tags:             server.Tags,
		Comment:          server.Comment,
		StatusSummary:    server.StatusSummary,
	}

	// 填充状态信息（如果存在）
	if server.CPUPercent != nil || server.MemUsed != nil {
		pbStatus := &pb.SystemStatus{}
		
		if server.CPUPercent != nil {
			pbStatus.CpuPercent = *server.CPUPercent
		}
		if server.MemUsed != nil {
			pbStatus.MemUsed = *server.MemUsed
		}
		if server.MemTotal != nil {
			pbStatus.MemTotal = *server.MemTotal
		}
		if server.Load1 != nil {
			pbStatus.Load1 = *server.Load1
		}
		if server.Load5 != nil {
			pbStatus.Load5 = *server.Load5
		}
		if server.Load15 != nil {
			pbStatus.Load15 = *server.Load15
		}
		if server.CollectedAt != nil {
			pbStatus.CollectedAt = server.CollectedAt.Unix()
		}
		pbStatus.ErrorMessage = server.ErrorMessage

		// 转换磁盘项
		if server.DiskItems != nil {
			pbStatus.DiskItems = make([]*pb.DiskItem, 0, len(server.DiskItems))
			for _, item := range server.DiskItems {
				pbStatus.DiskItems = append(pbStatus.DiskItems, &pb.DiskItem{
					MountPoint: item.MountPoint,
					Used:       item.Used,
					Total:      item.Total,
					Percent:    item.Percent,
				})
			}
		}

		pbServer.Status = pbStatus
	}

	// 设置上报时间
	if server.ReportedAt != nil {
		pbServer.ReportedAt = server.ReportedAt.Unix()
	}

	return pbServer
}

// convertToGRPCError 转换业务错误为 gRPC 错误
func convertToGRPCError(err error) error {
	if err == nil {
		return nil
	}

	// 检查是否是 errorc 错误
	if errorc.IsNotFound(err) {
		return status.Error(codes.NotFound, err.Error())
	}

	// 尝试解析为自定义 Error 类型
	customErr := errorc.ParseError(err)
	if customErr != nil && customErr.ErrorCode != nil {
		switch customErr.ErrorCode {
		case errorc.ErrorCodeValid:
			return status.Error(codes.InvalidArgument, err.Error())
		case errorc.ErrorCodeNoAuth:
			return status.Error(codes.Unauthenticated, err.Error())
		case errorc.ErrorCodeForbidden:
			return status.Error(codes.PermissionDenied, err.Error())
		case errorc.ErrorCodeNotFound:
			return status.Error(codes.NotFound, err.Error())
		case errorc.ErrorCodeDB, errorc.ErrorCodeThird:
			return status.Error(codes.Internal, err.Error())
		}
	}

	// 默认返回内部错误
	return status.Error(codes.Internal, err.Error())
}


