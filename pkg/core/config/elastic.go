package config

import (
	"strings"

	"github.com/olivere/elastic/v7"
)

type ES struct {
	Host     string `yaml:"host"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func InitES(es ES, proxyConfig ProxyConfig) (*elastic.Client, error) {
	options := []elastic.ClientOptionFunc{
		elastic.SetSniff(false),
		elastic.SetURL(strings.Split(es.Host, ",")...),
	}

	// 如果有认证信息，添加基本认证
	if es.Username != "" && es.Password != "" {
		options = append(options, elastic.SetBasicAuth(es.Username, es.Password))
	}

	// 如果启用代理，配置HTTP客户端使用代理
	if proxyConfig.Enabled {
		httpClient := proxyConfig.GetHTTPClient()
		options = append(options, elastic.SetHttpClient(httpClient))
	}

	return elastic.NewClient(options...)
}
