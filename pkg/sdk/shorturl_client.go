package sdk

import (
	"context"
	"encoding/json"

	shorturlpb "github.com/xsxdot/aio/system/shorturl/api/proto"

	"google.golang.org/grpc"
)

// ShortURLClient 短网址客户端
type ShortURLClient struct {
	service shorturlpb.ShortURLServiceClient
}

// newShortURLClient 创建短网址客户端
func newShortURLClient(conn *grpc.ClientConn) *ShortURLClient {
	return &ShortURLClient{
		service: shorturlpb.NewShortURLServiceClient(conn),
	}
}

// CreateShortLinkRequest 创建短链接请求（SDK 简化版）
type CreateShortLinkRequest struct {
	DomainID     int64
	TargetType   string                 // 跳转类型，支持："URL"（普通URL跳转）、"URL_SCHEME"（URL Scheme如weixin://）
	TargetConfig map[string]interface{} // 自动转 JSON，对于URL类型需包含"url"字段，对于URL_SCHEME需包含"url"或"schemeUrl"字段
	ExpiresAt    int64                  // unix 时间戳（可选）
	Password     string                 // 密码（可选）
	MaxVisits    int64                  // 最大访问次数（可选）
	CodeLength   int32                  // 短码长度（可选）
	CustomCode   string                 // 自定义短码（可选）
	Comment      string                 // 备注（可选）
}

// CreateShortLinkResponse 创建短链接响应
type CreateShortLinkResponse struct {
	ID       int64
	Code     string
	ShortURL string
}

// CreateShortLink 创建短链接
func (c *ShortURLClient) CreateShortLink(ctx context.Context, req *CreateShortLinkRequest) (*CreateShortLinkResponse, error) {
	// 序列化 TargetConfig
	targetConfigJSON, err := json.Marshal(req.TargetConfig)
	if err != nil {
		return nil, WrapError(err, "marshal target config failed")
	}

	pbReq := &shorturlpb.CreateShortLinkRequest{
		DomainId:         req.DomainID,
		TargetType:       req.TargetType,
		TargetConfigJson: string(targetConfigJSON),
		ExpiresAt:        req.ExpiresAt,
		Password:         req.Password,
		MaxVisits:        req.MaxVisits,
		CodeLength:       req.CodeLength,
		CustomCode:       req.CustomCode,
		Comment:          req.Comment,
	}

	resp, err := c.service.CreateShortLink(ctx, pbReq)
	if err != nil {
		return nil, WrapError(err, "create short link failed")
	}

	return &CreateShortLinkResponse{
		ID:       resp.Id,
		Code:     resp.Code,
		ShortURL: resp.ShortUrl,
	}, nil
}

// ShortLinkInfo 短链接信息
type ShortLinkInfo struct {
	ID           int64
	DomainID     int64
	Domain       string
	Code         string
	ShortURL     string
	TargetType   string
	TargetConfig map[string]interface{} // 自动解析 JSON
	ExpiresAt    int64
	MaxVisits    int64
	VisitCount   int64
	SuccessCount int64
	HasPassword  bool
	Enabled      bool
	Comment      string
	CreatedAt    int64
	UpdatedAt    int64
}

// GetShortLink 获取短链接详情
func (c *ShortURLClient) GetShortLink(ctx context.Context, id int64) (*ShortLinkInfo, error) {
	req := &shorturlpb.GetShortLinkRequest{
		Id: id,
	}

	resp, err := c.service.GetShortLink(ctx, req)
	if err != nil {
		return nil, WrapError(err, "get short link failed")
	}

	return c.convertShortLinkInfo(resp), nil
}

// ListShortLinks 查询短链接列表
func (c *ShortURLClient) ListShortLinks(ctx context.Context, domainID int64, page, size int32) ([]*ShortLinkInfo, int64, error) {
	req := &shorturlpb.ListShortLinksRequest{
		DomainId: domainID,
		Page:     page,
		Size:     size,
	}

	resp, err := c.service.ListShortLinks(ctx, req)
	if err != nil {
		return nil, 0, WrapError(err, "list short links failed")
	}

	links := make([]*ShortLinkInfo, len(resp.Links))
	for i, link := range resp.Links {
		links[i] = c.convertShortLinkInfo(link)
	}

	return links, resp.Total, nil
}

// ResolveResponse 解析短链接响应
type ResolveResponse struct {
	TargetType   string
	TargetConfig map[string]interface{} // 自动解析 JSON
	Action       string                 // "redirect" 或 "landing_page"
}

// Resolve 解析短链接（返回目标配置和建议动作）
func (c *ShortURLClient) Resolve(ctx context.Context, host, code, password string) (*ResolveResponse, error) {
	req := &shorturlpb.ResolveRequest{
		Host:     host,
		Code:     code,
		Password: password,
	}

	resp, err := c.service.Resolve(ctx, req)
	if err != nil {
		return nil, WrapError(err, "resolve short link failed")
	}

	// 解析 TargetConfig JSON
	var targetConfig map[string]interface{}
	if resp.TargetConfigJson != "" {
		if err := json.Unmarshal([]byte(resp.TargetConfigJson), &targetConfig); err != nil {
			return nil, WrapError(err, "unmarshal target config failed")
		}
	}

	return &ResolveResponse{
		TargetType:   resp.TargetType,
		TargetConfig: targetConfig,
		Action:       resp.Action,
	}, nil
}

// ReportSuccess 上报跳转成功（无鉴权）
func (c *ShortURLClient) ReportSuccess(ctx context.Context, code, eventID string, attrs map[string]interface{}) error {
	// 序列化 attrs
	attrsJSON := ""
	if len(attrs) > 0 {
		data, err := json.Marshal(attrs)
		if err != nil {
			return WrapError(err, "marshal attrs failed")
		}
		attrsJSON = string(data)
	}

	req := &shorturlpb.ReportSuccessRequest{
		Code:      code,
		EventId:   eventID,
		AttrsJson: attrsJSON,
	}

	_, err := c.service.ReportSuccess(ctx, req)
	if err != nil {
		return WrapError(err, "report success failed")
	}

	return nil
}

// convertShortLinkInfo 转换 protobuf 短链接信息为 SDK 结构
func (c *ShortURLClient) convertShortLinkInfo(pbInfo *shorturlpb.ShortLinkInfo) *ShortLinkInfo {
	if pbInfo == nil {
		return nil
	}

	// 解析 TargetConfig JSON
	var targetConfig map[string]interface{}
	if pbInfo.TargetConfigJson != "" {
		_ = json.Unmarshal([]byte(pbInfo.TargetConfigJson), &targetConfig)
	}

	return &ShortLinkInfo{
		ID:           pbInfo.Id,
		DomainID:     pbInfo.DomainId,
		Domain:       pbInfo.Domain,
		Code:         pbInfo.Code,
		ShortURL:     pbInfo.ShortUrl,
		TargetType:   pbInfo.TargetType,
		TargetConfig: targetConfig,
		ExpiresAt:    pbInfo.ExpiresAt,
		MaxVisits:    pbInfo.MaxVisits,
		VisitCount:   pbInfo.VisitCount,
		SuccessCount: pbInfo.SuccessCount,
		HasPassword:  pbInfo.HasPassword,
		Enabled:      pbInfo.Enabled,
		Comment:      pbInfo.Comment,
		CreatedAt:    pbInfo.CreatedAt,
		UpdatedAt:    pbInfo.UpdatedAt,
	}
}
