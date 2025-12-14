package dto

import "time"

// CertificateDTO 证书对外 DTO
type CertificateDTO struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Domain          string    `json:"domain"`
	Email           string    `json:"email"`
	Status          string    `json:"status"`
	ExpiresAt       time.Time `json:"expiresAt"`
	IssuedAt        time.Time `json:"issuedAt"`
	AutoRenew       bool      `json:"autoRenew"`
	AutoDeploy      bool      `json:"autoDeploy"`
	RenewBeforeDays int       `json:"renewBeforeDays"`
	Description     string    `json:"description"`
}

// IssueCertificateReq 签发证书请求
type IssueCertificateReq struct {
	Name            string `json:"name"`
	Domain          string `json:"domain" validate:"required"`
	Email           string `json:"email" validate:"required,email"`
	DnsCredentialID int64  `json:"dnsCredentialId" validate:"required"`
	RenewBeforeDays int    `json:"renewBeforeDays"`
	AutoRenew       bool   `json:"autoRenew"`
	AutoDeploy      bool   `json:"autoDeploy"` // 如果为 true，会根据证书域名自动匹配部署目标
	Description     string `json:"description"`
	UseStaging      bool   `json:"useStaging"`
}

// DeployCertificateReq 部署证书请求
type DeployCertificateReq struct {
	CertificateID int64   `json:"certificateId" validate:"required"`
	TargetIDs     []int64 `json:"targetIds" validate:"required"`
	TriggerType   string  `json:"triggerType"`
}

// DeployToLocalReq 部署证书到本地请求
type DeployToLocalReq struct {
	CertificateID int64  `json:"certificateId" validate:"required"`
	CertPath      string `json:"certPath" validate:"required"`
	KeyPath       string `json:"keyPath" validate:"required"`
}

// CertificateContentDTO 证书内容 DTO（用于部署）
type CertificateContentDTO struct {
	FullchainPem string `json:"fullchainPem"`
	PrivkeyPem   string `json:"privkeyPem"`
}

// FindCertificateReq 查找证书请求
type FindCertificateReq struct {
	Domain string `json:"domain" validate:"required"`
}

