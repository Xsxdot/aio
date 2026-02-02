package model

import (
	"time"
	"github.com/xsxdot/aio/pkg/core/model/common"
)

// Certificate SSL 证书模型
type Certificate struct {
	common.Model
	Name            string            `gorm:"size:100;not null;index" json:"name" comment:"证书名称"`
	Domain          string            `gorm:"size:200;not null;index" json:"domain" comment:"域名（支持通配符，如 *.a.com）"`
	Email           string            `gorm:"size:200;not null" json:"email" comment:"Let's Encrypt 注册邮箱"`
	DnsCredentialID uint              `gorm:"not null;index" json:"dns_credential_id" comment:"DNS 凭证 ID"`
	Status          CertificateStatus `gorm:"size:50;not null;index;default:'pending'" json:"status" comment:"证书状态"`
	ExpiresAt       *time.Time        `gorm:"index" json:"expires_at" comment:"过期时间"`
	IssuedAt        *time.Time        `json:"issued_at" comment:"签发时间"`
	LastRenewAt     *time.Time        `json:"last_renew_at" comment:"最后续期时间"`
	RenewBeforeDays int               `gorm:"default:30;not null" json:"renew_before_days" comment:"提前多少天续期"`
	FullchainPem    string            `gorm:"type:longtext" json:"fullchain_pem" comment:"完整证书链 PEM（Nginx fullchain.pem）"`
	PrivkeyPem      string            `gorm:"type:longtext" json:"privkey_pem" comment:"私钥 PEM（Nginx privkey.pem）"`
	AcmeAccountURL  string            `gorm:"size:500" json:"acme_account_url" comment:"ACME 账户 URL"`
	AcmeAccountKey  string            `gorm:"type:text" json:"acme_account_key" comment:"ACME 账户私钥（加密存储）"`
	CertURL         string            `gorm:"size:500" json:"cert_url" comment:"证书资源 URL"`
	AutoRenew       int               `gorm:"default:1;not null" json:"auto_renew" comment:"是否自动续期：1=是，0=否"`
	AutoDeploy      int               `gorm:"default:1;not null" json:"auto_deploy" comment:"续期后是否自动部署：1=是，0=否"`
	LastError       string            `gorm:"type:text" json:"last_error" comment:"最后一次错误信息"`
	Description     string            `gorm:"size:500" json:"description" comment:"证书描述"`
}

// TableName 指定表名
func (Certificate) TableName() string {
	return "ssl_certificates"
}
