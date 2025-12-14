package config

type JwtConfig struct {
	Secret      string `yaml:"secret" json:"secret,omitempty"`
	AdminSecret string `yaml:"admin-secret" json:"admin-secret,omitempty"`
	ExpireTime  int    `yaml:"expire-time" json:"expire-time,omitempty"`
}
