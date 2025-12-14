package model

// DnsProvider DNS 服务商类型
type DnsProvider string

const (
	DnsProviderAliDNS       DnsProvider = "alidns"       // 阿里云 DNS
	DnsProviderTencentCloud DnsProvider = "tencentcloud" // 腾讯云 DNS
	DnsProviderDNSPod       DnsProvider = "dnspod"       // DNSPod（兼容别名，映射到 tencentcloud）
)

// CertificateStatus 证书状态
type CertificateStatus string

const (
	CertificateStatusPending  CertificateStatus = "pending"  // 待签发
	CertificateStatusIssuing  CertificateStatus = "issuing"  // 签发中
	CertificateStatusActive   CertificateStatus = "active"   // 已签发有效
	CertificateStatusRenewing CertificateStatus = "renewing" // 续期中
	CertificateStatusExpired  CertificateStatus = "expired"  // 已过期
	CertificateStatusFailed   CertificateStatus = "failed"   // 签发失败
)

// DeployTargetType 部署目标类型
type DeployTargetType string

const (
	DeployTargetTypeLocal     DeployTargetType = "local"      // 本机文件
	DeployTargetTypeSSH       DeployTargetType = "ssh"        // SSH 远端
	DeployTargetTypeAliyunCAS DeployTargetType = "aliyun_cas" // 阿里云证书服务
)

// DeployStatus 部署状态
type DeployStatus string

const (
	DeployStatusPending   DeployStatus = "pending"   // 待部署
	DeployStatusDeploying DeployStatus = "deploying" // 部署中
	DeployStatusSuccess   DeployStatus = "success"   // 部署成功
	DeployStatusFailed    DeployStatus = "failed"    // 部署失败
	DeployStatusPartial   DeployStatus = "partial"   // 部分成功
)
