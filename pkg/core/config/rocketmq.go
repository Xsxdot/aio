package config

type RocketMQ struct {
	NameServer   string `yaml:"name-server" json:"name-server"`
	NameSpace    string `yaml:"name-space" json:"name-space"`
	AccessKey    string `yaml:"access-key" json:"access-key"`
	AccessSecret string `yaml:"access-secret" json:"access-secret"`
	Enable       bool   `yaml:"enable" json:"enable"`
	Endpoint     string `yaml:"endpoint" json:"endpoint"`
}
