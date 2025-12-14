package config

// WechatConfig 微信配置结构体
type WechatConfig struct {
	Miniprogram MiniprogramConfig `yaml:"miniprogram"` // 小程序配置
	Payment     PaymentConfig     `yaml:"payment"`     // 支付配置
}

// MiniprogramConfig 小程序配置
type MiniprogramConfig struct {
	AppID     string `yaml:"app-id"`     // 小程序AppID
	AppSecret string `yaml:"app-secret"` // 小程序AppSecret
}

// PaymentConfig 支付配置
type PaymentConfig struct {
	MchID         string `yaml:"mch-id"`         // 商户号
	APIv3Key      string `yaml:"api-v3-key"`     // APIv3密钥
	SerialNo      string `yaml:"serial-no"`      // 证书序列号
	CertFile      string `yaml:"cert-file"`      // p12证书文件路径
	CertPassword  string `yaml:"cert-password"`  // p12证书密码（通常是商户号）
	NotifyURL     string `yaml:"notify-url"`     // 支付回调地址
}

