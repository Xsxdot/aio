package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	pb "github.com/xsxdot/aio/system/shorturl/api/proto"
	internalapp "github.com/xsxdot/aio/system/shorturl/internal/app"
	"github.com/xsxdot/aio/system/shorturl/internal/model"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ShortURLService gRPC 短网址服务实现
type ShortURLService struct {
	pb.UnimplementedShortURLServiceServer
	app *internalapp.App
	err *errorc.ErrorBuilder
	log *logger.Log
}

// NewShortURLService 创建短网址服务实例
func NewShortURLService(app *internalapp.App, log *logger.Log) *ShortURLService {
	return &ShortURLService{
		app: app,
		err: errorc.NewErrorBuilder("ShortURLGRPCService"),
		log: log,
	}
}

// ServiceName 返回服务名称
func (s *ShortURLService) ServiceName() string {
	return "shorturl.v1.ShortURLService"
}

// ServiceVersion 返回服务版本
func (s *ShortURLService) ServiceVersion() string {
	return "v1.0.0"
}

// RegisterService 注册服务到 gRPC 服务器
func (s *ShortURLService) RegisterService(server *grpc.Server) error {
	pb.RegisterShortURLServiceServer(server, s)
	return nil
}

// CreateShortLink 创建短链接
func (s *ShortURLService) CreateShortLink(ctx context.Context, req *pb.CreateShortLinkRequest) (*pb.CreateShortLinkResponse, error) {
	// 解析 target_config JSON
	var targetConfig map[string]interface{}
	if err := json.Unmarshal([]byte(req.TargetConfigJson), &targetConfig); err != nil {
		return nil, status.Error(codes.InvalidArgument, "target_config_json 格式错误")
	}

	// 构建创建请求
	createReq := &internalapp.CreateShortLinkRequest{
		DomainID:     req.DomainId,
		TargetType:   model.TargetType(req.TargetType),
		TargetConfig: targetConfig,
		CodeLength:   int(req.CodeLength),
		CustomCode:   req.CustomCode,
		Comment:      req.Comment,
	}

	if req.ExpiresAt > 0 {
		t := time.Unix(req.ExpiresAt, 0)
		createReq.ExpiresAt = &t
	}
	if req.Password != "" {
		createReq.Password = req.Password
	}
	if req.MaxVisits > 0 {
		createReq.MaxVisits = &req.MaxVisits
	}

	// 创建短链接
	link, err := s.app.CreateShortLink(ctx, createReq)
	if err != nil {
		return nil, convertToGRPCError(err)
	}

	// 查询域名构建完整URL
	domain, err := s.app.DomainService.FindById(ctx, link.DomainID)
	if err != nil {
		return nil, convertToGRPCError(err)
	}

	return &pb.CreateShortLinkResponse{
		Id:       link.ID,
		Code:     link.Code,
		ShortUrl: fmt.Sprintf("https://%s/%s", domain.Domain, link.Code),
	}, nil
}

// GetShortLink 获取短链接详情
func (s *ShortURLService) GetShortLink(ctx context.Context, req *pb.GetShortLinkRequest) (*pb.ShortLinkInfo, error) {
	link, err := s.app.LinkService.FindById(ctx, req.Id)
	if err != nil {
		return nil, convertToGRPCError(err)
	}

	domain, err := s.app.DomainService.FindById(ctx, link.DomainID)
	if err != nil {
		return nil, convertToGRPCError(err)
	}

	return convertToProtoShortLinkInfo(link, domain), nil
}

// ListShortLinks 查询短链接列表
func (s *ShortURLService) ListShortLinks(ctx context.Context, req *pb.ListShortLinksRequest) (*pb.ListShortLinksResponse, error) {
	pageNum := int(req.Page)
	if pageNum <= 0 {
		pageNum = 1
	}
	pageSize := int(req.Size)
	if pageSize <= 0 {
		pageSize = 20
	}

	links, total, err := s.app.LinkService.Dao.ListByDomainWithPage(ctx, req.DomainId, pageNum, pageSize)
	if err != nil {
		return nil, convertToGRPCError(err)
	}

	domain, err := s.app.DomainService.FindById(ctx, req.DomainId)
	if err != nil {
		return nil, convertToGRPCError(err)
	}

	pbLinks := make([]*pb.ShortLinkInfo, 0, len(links))
	for _, link := range links {
		pbLinks = append(pbLinks, convertToProtoShortLinkInfo(link, domain))
	}

	return &pb.ListShortLinksResponse{
		Links: pbLinks,
		Total: total,
	}, nil
}

// Resolve 解析短链接
func (s *ShortURLService) Resolve(ctx context.Context, req *pb.ResolveRequest) (*pb.ResolveResponse, error) {
	link, _, err := s.app.ResolveShortLink(ctx, req.Host, req.Code, req.Password)
	if err != nil {
		return nil, convertToGRPCError(err)
	}

	targetConfigJSON, _ := json.Marshal(link.TargetConfig)
	action := "landing_page"
	if link.TargetType == model.TargetTypeURL {
		action = "redirect"
	}

	return &pb.ResolveResponse{
		TargetType:       string(link.TargetType),
		TargetConfigJson: string(targetConfigJSON),
		Action:           action,
	}, nil
}

// ReportSuccess 上报跳转成功
func (s *ShortURLService) ReportSuccess(ctx context.Context, req *pb.ReportSuccessRequest) (*pb.ReportSuccessResponse, error) {
	// 解析 attrs JSON
	var attrs map[string]interface{}
	if req.AttrsJson != "" {
		if err := json.Unmarshal([]byte(req.AttrsJson), &attrs); err != nil {
			return nil, status.Error(codes.InvalidArgument, "attrs_json 格式错误")
		}
	}

	if err := s.app.ReportShortLinkSuccess(ctx, req.Code, req.EventId, attrs); err != nil {
		return nil, convertToGRPCError(err)
	}

	return &pb.ReportSuccessResponse{
		Success: true,
		Message: "上报成功",
	}, nil
}

// convertToProtoShortLinkInfo 转换为 proto 格式
func convertToProtoShortLinkInfo(link *model.ShortLink, domain *model.ShortDomain) *pb.ShortLinkInfo {
	targetConfigJSON, _ := json.Marshal(link.TargetConfig)

	pbLink := &pb.ShortLinkInfo{
		Id:               link.ID,
		DomainId:         link.DomainID,
		Domain:           domain.Domain,
		Code:             link.Code,
		ShortUrl:         fmt.Sprintf("https://%s/%s", domain.Domain, link.Code),
		TargetType:       string(link.TargetType),
		TargetConfigJson: string(targetConfigJSON),
		VisitCount:       link.VisitCount,
		SuccessCount:     link.SuccessCount,
		HasPassword:      link.HasPassword(),
		Enabled:          link.Enabled,
		Comment:          link.Comment,
		CreatedAt:        link.CreatedAt.Unix(),
		UpdatedAt:        link.UpdatedAt.Unix(),
	}

	if link.ExpiresAt != nil {
		pbLink.ExpiresAt = link.ExpiresAt.Unix()
	}
	if link.MaxVisits != nil {
		pbLink.MaxVisits = *link.MaxVisits
	}

	return pbLink
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


