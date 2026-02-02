package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/system/ssl/api/dto"
	"github.com/xsxdot/aio/system/ssl/internal/app"
	"github.com/xsxdot/aio/system/ssl/internal/model"
)

// SslClient SSL 证书组件对外客户端
// 供其他组件调用 SSL 证书能力（如查询证书状态等）
type SslClient struct {
	app *app.App
	err *errorc.ErrorBuilder
}

// NewSslClient 创建 SSL 证书客户端实例
func NewSslClient(appInstance *app.App) *SslClient {
	return &SslClient{
		app: appInstance,
		err: errorc.NewErrorBuilder("SslClient"),
	}
}

// IssueCertificate 签发新证书
func (c *SslClient) IssueCertificate(ctx context.Context, req *dto.IssueCertificateReq) (*dto.CertificateDTO, error) {
	internalReq := &app.IssueCertificateRequest{
		Name:            req.Name,
		Domain:          req.Domain,
		Email:           req.Email,
		DnsCredentialID: uint(req.DnsCredentialID),
		RenewBeforeDays: req.RenewBeforeDays,
		AutoRenew:       req.AutoRenew,
		AutoDeploy:      req.AutoDeploy, // 如果为 true，会根据证书域名自动匹配部署目标
		Description:     req.Description,
		UseStaging:      req.UseStaging,
	}

	cert, err := c.app.IssueCertificate(ctx, internalReq)
	if err != nil {
		return nil, err
	}

	return c.toDTO(cert), nil
}

// RenewCertificate 续期证书
func (c *SslClient) RenewCertificate(ctx context.Context, certificateID int64) error {
	return c.app.RenewCertificate(ctx, uint(certificateID))
}

// GetCertificate 获取证书详情
func (c *SslClient) GetCertificate(ctx context.Context, id int64) (*dto.CertificateDTO, error) {
	cert, err := c.app.GetCertificate(ctx, uint(id))
	if err != nil {
		return nil, err
	}
	return c.toDTO(cert), nil
}

// ListCertificates 查询证书列表
func (c *SslClient) ListCertificates(ctx context.Context, page, pageSize int) ([]*dto.CertificateDTO, int64, error) {
	certs, total, err := c.app.ListCertificates(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*dto.CertificateDTO, len(certs))
	for i := range certs {
		result[i] = c.toDTO(&certs[i])
	}
	return result, total, nil
}

// DeployCertificateToTargets 部署证书到多个目标
func (c *SslClient) DeployCertificateToTargets(ctx context.Context, req *dto.DeployCertificateReq) error {
	targetIDs := make([]uint, len(req.TargetIDs))
	for i, id := range req.TargetIDs {
		targetIDs[i] = uint(id)
	}

	triggerType := req.TriggerType
	if triggerType == "" {
		triggerType = "manual"
	}

	return c.app.DeployCertificateToTargets(ctx, uint(req.CertificateID), targetIDs, triggerType)
}

// GetCertificateContent 获取证书内容（用于部署）
func (c *SslClient) GetCertificateContent(ctx context.Context, id int64) (*dto.CertificateContentDTO, error) {
	cert, err := c.app.GetCertificate(ctx, uint(id))
	if err != nil {
		return nil, err
	}

	return &dto.CertificateContentDTO{
		FullchainPem: cert.FullchainPem,
		PrivkeyPem:   cert.PrivkeyPem,
	}, nil
}

// DeployToLocal 部署证书到本机指定路径
func (c *SslClient) DeployToLocal(ctx context.Context, req *dto.DeployToLocalReq) error {
	// 获取证书内容
	cert, err := c.app.GetCertificate(ctx, uint(req.CertificateID))
	if err != nil {
		return err
	}

	// 确保目录存在
	certDir := filepath.Dir(req.CertPath)
	keyDir := filepath.Dir(req.KeyPath)

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return c.err.New("创建证书目录失败", err)
	}
	if err := os.MkdirAll(keyDir, 0755); err != nil {
		return c.err.New("创建密钥目录失败", err)
	}

	// 写入证书文件
	if err := os.WriteFile(req.CertPath, []byte(cert.FullchainPem), 0644); err != nil {
		return c.err.New("写入证书文件失败", err)
	}

	// 写入密钥文件
	if err := os.WriteFile(req.KeyPath, []byte(cert.PrivkeyPem), 0600); err != nil {
		return c.err.New("写入密钥文件失败", err)
	}

	return nil
}

// FindMatchingCertificate 查找匹配域名的可用证书
// 按优先级：精确匹配 > 通配符匹配
// 只返回未过期且状态正常的证书
func (c *SslClient) FindMatchingCertificate(ctx context.Context, domain string) (*dto.CertificateDTO, error) {
	// 获取所有有效证书
	certs, _, err := c.app.ListCertificates(ctx, 1, 1000)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	var exactMatch *model.Certificate
	var wildcardMatch *model.Certificate

	for i := range certs {
		cert := &certs[i]

		// 检查状态和过期时间
		if cert.Status != model.CertificateStatusActive {
			continue
		}
		if cert.ExpiresAt == nil || cert.ExpiresAt.Before(now) {
			continue
		}

		// 精确匹配
		if cert.Domain == domain {
			exactMatch = cert
			break
		}

		// 通配符匹配 (*.example.com 匹配 sub.example.com)
		if strings.HasPrefix(cert.Domain, "*.") {
			wildcardBase := cert.Domain[2:] // 去掉 "*."
			if strings.HasSuffix(domain, "."+wildcardBase) || domain == wildcardBase {
				if wildcardMatch == nil {
					wildcardMatch = cert
				}
			}
		}
	}

	// 优先返回精确匹配
	if exactMatch != nil {
		return c.toDTO(exactMatch), nil
	}

	// 其次返回通配符匹配
	if wildcardMatch != nil {
		return c.toDTO(wildcardMatch), nil
	}

	return nil, nil // 未找到匹配证书
}

// EnsureCertificate 确保域名有可用证书
// 如果找到匹配的现有证书则复用，否则申请新证书
func (c *SslClient) EnsureCertificate(ctx context.Context, domain string, issueReq *dto.IssueCertificateReq) (*dto.CertificateDTO, bool, error) {
	// 1. 尝试查找已有证书
	existing, err := c.FindMatchingCertificate(ctx, domain)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, true, nil // 复用已有证书
	}

	// 2. 没有找到，签发新证书
	if issueReq == nil {
		return nil, false, c.err.New("未找到匹配证书且未提供签发请求", nil).ValidWithCtx()
	}

	// 确保域名在请求中
	if issueReq.Domain == "" {
		issueReq.Domain = domain
	}

	newCert, err := c.IssueCertificate(ctx, issueReq)
	if err != nil {
		return nil, false, err
	}

	return newCert, false, nil // 新签发的证书
}

// toDTO 转换为对外 DTO
func (c *SslClient) toDTO(cert *model.Certificate) *dto.CertificateDTO {
	if cert == nil {
		return nil
	}

	var expiresAt, issuedAt time.Time
	if cert.ExpiresAt != nil {
		expiresAt = *cert.ExpiresAt
	}
	if cert.IssuedAt != nil {
		issuedAt = *cert.IssuedAt
	}

	return &dto.CertificateDTO{
		ID:              cert.ID,
		Name:            cert.Name,
		Domain:          cert.Domain,
		Email:           cert.Email,
		Status:          string(cert.Status),
		ExpiresAt:       expiresAt,
		IssuedAt:        issuedAt,
		AutoRenew:       cert.AutoRenew == 1,
		AutoDeploy:      cert.AutoDeploy == 1,
		RenewBeforeDays: cert.RenewBeforeDays,
		Description:     cert.Description,
	}
}
