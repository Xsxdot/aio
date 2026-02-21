package config

type JwtConfig struct {
	Secret       string `yaml:"secret" json:"secret,omitempty"`
	AdminSecret  string `yaml:"admin-secret" json:"admin-secret,omitempty"`
	ClientSecret string `yaml:"client-secret" json:"client-secret,omitempty"`
	ExpireTime   int    `yaml:"expire-time" json:"expire-time,omitempty"`
}
