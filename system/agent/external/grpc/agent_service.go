package grpc

import (
	"context"
	"time"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	pb "xiaozhizhang/system/agent/api/proto"
	"xiaozhizhang/system/agent/internal/service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AgentGRPCService Agent gRPC 服务实现
type AgentGRPCService struct {
	pb.UnimplementedAgentServiceServer
	nginxSvc   *service.NginxService
	systemdSvc *service.SystemdService
	sslSvc     *service.SSLService
	log        *logger.Log
	err        *errorc.ErrorBuilder
}

// NewAgentGRPCService 创建 Agent gRPC 服务实例
func NewAgentGRPCService(nginxSvc *service.NginxService, systemdSvc *service.SystemdService, sslSvc *service.SSLService, log *logger.Log) *AgentGRPCService {
	return &AgentGRPCService{
		nginxSvc:   nginxSvc,
		systemdSvc: systemdSvc,
		sslSvc:     sslSvc,
		log:        log.WithEntryName("AgentGRPCService"),
		err:        errorc.NewErrorBuilder("AgentGRPCService"),
	}
}

// ServiceName 返回服务名称
func (s *AgentGRPCService) ServiceName() string {
	return "agent.v1.AgentService"
}

// ServiceVersion 返回服务版本
func (s *AgentGRPCService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册服务到 gRPC 服务器
func (s *AgentGRPCService) RegisterService(server *grpc.Server) error {
	pb.RegisterAgentServiceServer(server, s)
	return nil
}

// ==================== Nginx 配置管理 ====================

func (s *AgentGRPCService) PutNginxConfig(ctx context.Context, req *pb.PutNginxConfigRequest) (*pb.PutNginxConfigResponse, error) {
	if req.Name == "" || req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "name 和 content 不能为空")
	}

	validate := req.Validate
	reload := req.Reload

	validateOut, reloadOut, err := s.nginxSvc.PutConfig(ctx, req.Name, req.Content, validate, reload)
	if err != nil {
		s.log.WithErr(err).WithField("name", req.Name).Error("创建/更新 nginx 配置失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.PutNginxConfigResponse{
		Success:         true,
		Message:         "配置已保存",
		ValidateOutput:  validateOut,
		ReloadOutput:    reloadOut,
	}, nil
}

func (s *AgentGRPCService) GetNginxConfig(ctx context.Context, req *pb.GetNginxConfigRequest) (*pb.GetNginxConfigResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}

	info, err := s.nginxSvc.GetConfig(req.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &pb.GetNginxConfigResponse{
		Name:        info.Name,
		Content:     info.Content,
		Description: info.Description,
		ModTime:     info.ModTime.Unix(),
	}, nil
}

func (s *AgentGRPCService) DeleteNginxConfig(ctx context.Context, req *pb.DeleteNginxConfigRequest) (*pb.DeleteNginxConfigResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}

	validate := req.Validate
	reload := req.Reload

	if err := s.nginxSvc.DeleteConfig(ctx, req.Name, validate, reload); err != nil {
		s.log.WithErr(err).WithField("name", req.Name).Error("删除 nginx 配置失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.DeleteNginxConfigResponse{
		Success: true,
		Message: "配置已删除",
	}, nil
}

func (s *AgentGRPCService) ListNginxConfigs(ctx context.Context, req *pb.ListNginxConfigsRequest) (*pb.ListNginxConfigsResponse, error) {
	configs, err := s.nginxSvc.ListConfigs(req.Keyword)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var items []*pb.NginxConfigItem
	for _, cfg := range configs {
		items = append(items, &pb.NginxConfigItem{
			Name:        cfg.Name,
			Description: cfg.Description,
			ModTime:     cfg.ModTime.Unix(),
		})
	}

	return &pb.ListNginxConfigsResponse{
		Configs: items,
	}, nil
}

func (s *AgentGRPCService) ValidateNginxConfig(ctx context.Context, req *pb.ValidateNginxConfigRequest) (*pb.ValidateNginxConfigResponse, error) {
	output, err := s.nginxSvc.Validate(ctx)
	if err != nil {
		return &pb.ValidateNginxConfigResponse{
			Success: false,
			Output:  output,
		}, nil
	}

	return &pb.ValidateNginxConfigResponse{
		Success: true,
		Output:  output,
	}, nil
}

func (s *AgentGRPCService) ReloadNginx(ctx context.Context, req *pb.ReloadNginxRequest) (*pb.ReloadNginxResponse, error) {
	output, err := s.nginxSvc.Reload(ctx)
	if err != nil {
		return &pb.ReloadNginxResponse{
			Success: false,
			Output:  output,
		}, nil
	}

	return &pb.ReloadNginxResponse{
		Success: true,
		Output:  output,
	}, nil
}

// ==================== Systemd 单元管理 ====================

func (s *AgentGRPCService) PutSystemdUnit(ctx context.Context, req *pb.PutSystemdUnitRequest) (*pb.PutSystemdUnitResponse, error) {
	if req.Name == "" || req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "name 和 content 不能为空")
	}

	daemonReload := req.DaemonReload

	if err := s.systemdSvc.PutUnit(ctx, req.Name, req.Content, daemonReload); err != nil {
		s.log.WithErr(err).WithField("name", req.Name).Error("创建/更新 systemd unit 失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.PutSystemdUnitResponse{
		Success: true,
		Message: "unit 已保存",
	}, nil
}

func (s *AgentGRPCService) GetSystemdUnit(ctx context.Context, req *pb.GetSystemdUnitRequest) (*pb.GetSystemdUnitResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}

	info, err := s.systemdSvc.GetUnit(req.Name)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return &pb.GetSystemdUnitResponse{
		Name:        info.Name,
		Content:     info.Content,
		Description: info.Description,
		ModTime:     info.ModTime.Unix(),
	}, nil
}

func (s *AgentGRPCService) DeleteSystemdUnit(ctx context.Context, req *pb.DeleteSystemdUnitRequest) (*pb.DeleteSystemdUnitResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}

	if err := s.systemdSvc.DeleteUnit(ctx, req.Name, req.StopService, req.DisableService, req.DaemonReload); err != nil {
		s.log.WithErr(err).WithField("name", req.Name).Error("删除 systemd unit 失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.DeleteSystemdUnitResponse{
		Success: true,
		Message: "unit 已删除",
	}, nil
}

func (s *AgentGRPCService) ListSystemdUnits(ctx context.Context, req *pb.ListSystemdUnitsRequest) (*pb.ListSystemdUnitsResponse, error) {
	units, err := s.systemdSvc.ListUnits(req.Keyword)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	var items []*pb.SystemdUnitItem
	for _, unit := range units {
		items = append(items, &pb.SystemdUnitItem{
			Name:        unit.Name,
			Description: unit.Description,
			ModTime:     unit.ModTime.Unix(),
		})
	}

	return &pb.ListSystemdUnitsResponse{
		Units: items,
	}, nil
}

func (s *AgentGRPCService) SystemdDaemonReload(ctx context.Context, req *pb.SystemdDaemonReloadRequest) (*pb.SystemdDaemonReloadResponse, error) {
	output, err := s.systemdSvc.DaemonReload(ctx)
	if err != nil {
		return &pb.SystemdDaemonReloadResponse{
			Success: false,
			Output:  output,
		}, nil
	}

	return &pb.SystemdDaemonReloadResponse{
		Success: true,
		Output:  output,
	}, nil
}

func (s *AgentGRPCService) SystemdServiceControl(ctx context.Context, req *pb.SystemdServiceControlRequest) (*pb.SystemdServiceControlResponse, error) {
	if req.Name == "" || req.Action == "" {
		return nil, status.Error(codes.InvalidArgument, "name 和 action 不能为空")
	}

	output, err := s.systemdSvc.ServiceControl(ctx, req.Name, req.Action)
	if err != nil {
		return &pb.SystemdServiceControlResponse{
			Success: false,
			Output:  output,
		}, nil
	}

	return &pb.SystemdServiceControlResponse{
		Success: true,
		Output:  output,
	}, nil
}

func (s *AgentGRPCService) GetSystemdServiceStatus(ctx context.Context, req *pb.GetSystemdServiceStatusRequest) (*pb.GetSystemdServiceStatusResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}

	statusInfo, err := s.systemdSvc.GetServiceStatus(ctx, req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetSystemdServiceStatusResponse{
		Name:              statusInfo.Name,
		Description:       statusInfo.Description,
		LoadState:         statusInfo.LoadState,
		ActiveState:       statusInfo.ActiveState,
		SubState:          statusInfo.SubState,
		UnitFileState:     statusInfo.UnitFileState,
		MainPid:           statusInfo.MainPID,
		ExecMainStartAt:   statusInfo.ExecMainStartAt,
		MemoryCurrent:     statusInfo.MemoryCurrent,
		Result:            statusInfo.Result,
	}, nil
}

func (s *AgentGRPCService) GetSystemdServiceLogs(ctx context.Context, req *pb.GetSystemdServiceLogsRequest) (*pb.GetSystemdServiceLogsResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name 不能为空")
	}

	lines := int(req.Lines)
	logLines, err := s.systemdSvc.GetServiceLogs(ctx, req.Name, lines, req.Since, req.Until)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.GetSystemdServiceLogsResponse{
		Name:     req.Name,
		LogLines: logLines,
	}, nil
}

// ==================== SSL 证书部署 ====================

func (s *AgentGRPCService) DeploySSLCertificate(ctx context.Context, req *pb.DeploySSLCertificateRequest) (*pb.DeploySSLCertificateResponse, error) {
	if req.BasePath == "" || req.FullchainPem == "" || req.PrivkeyPem == "" {
		return nil, status.Error(codes.InvalidArgument, "base_path、fullchain_pem 和 privkey_pem 不能为空")
	}

	fullchainPath, privkeyPath, err := s.sslSvc.DeployCertificate(
		req.BasePath,
		req.FullchainName,
		req.PrivkeyName,
		req.FullchainPem,
		req.PrivkeyPem,
		req.FileMode,
	)
	if err != nil {
		s.log.WithErr(err).Error("部署 SSL 证书失败")
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.DeploySSLCertificateResponse{
		Success:       true,
		Message:       "证书已部署",
		FullchainPath: fullchainPath,
		PrivkeyPath:   privkeyPath,
	}, nil
}

func (s *AgentGRPCService) ReloadService(ctx context.Context, req *pb.ReloadServiceRequest) (*pb.ReloadServiceResponse, error) {
	if req.ServiceType == "" {
		return nil, status.Error(codes.InvalidArgument, "service_type 不能为空")
	}

	output, err := s.sslSvc.ReloadService(ctx, req.ServiceType, req.ServiceName)
	if err != nil {
		return &pb.ReloadServiceResponse{
			Success: false,
			Output:  output,
		}, nil
	}

	return &pb.ReloadServiceResponse{
		Success: true,
		Output:  output,
	}, nil
}

// ==================== 健康检查 ====================

func (s *AgentGRPCService) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Status:    "ok",
		Version:   "1.0.0",
		Timestamp: time.Now().Unix(),
	}, nil
}

