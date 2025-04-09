package mq

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
)

// TLSConfigParams 表示TLS配置参数
type TLSConfigParams struct {
	// 服务端证书文件路径
	CertFile string
	// 服务端密钥文件路径
	KeyFile string
	// CA证书文件路径
	CAFile string
	// 是否需要验证客户端证书
	VerifyClients bool
	// 是否跳过验证服务器证书（仅用于测试环境，生产环境不应启用）
	InsecureSkipVerify bool
}

// LoadTLSConfig 从证书和密钥文件加载TLS配置
// 适用于客户端和服务器配置
func LoadTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	return LoadTLSConfigWithParams(&TLSConfigParams{
		CertFile: certFile,
		KeyFile:  keyFile,
		CAFile:   caFile,
	})
}

// LoadTLSConfigWithParams 使用更多参数从证书和密钥文件加载TLS配置
func LoadTLSConfigWithParams(params *TLSConfigParams) (*tls.Config, error) {
	if params == nil {
		return nil, errors.New("TLS配置参数不能为空")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// 如果提供了CA证书，则加载
	if params.CAFile != "" {
		// 加载CA证书
		caPool := x509.NewCertPool()
		caCert, err := ioutil.ReadFile(params.CAFile)
		if err != nil {
			return nil, fmt.Errorf("无法读取CA证书文件 %s: %v", params.CAFile, err)
		}

		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("无法解析CA证书: %s", params.CAFile)
		}

		tlsConfig.RootCAs = caPool

		// 如果需要验证客户端证书，设置客户端CA池
		if params.VerifyClients {
			tlsConfig.ClientCAs = caPool
			tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		}
	}

	// 如果同时提供了证书和密钥文件，则加载
	if params.CertFile != "" && params.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(params.CertFile, params.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("无法加载证书(%s)和密钥(%s): %v", params.CertFile, params.KeyFile, err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// 仅在明确指定时才跳过服务器证书验证（不安全）
	if params.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
}

// LoadServerTLSConfig 加载NATS服务器TLS配置
func LoadServerTLSConfig(certFile, keyFile, caFile string, verifyClients bool) (*tls.Config, error) {
	return LoadTLSConfigWithParams(&TLSConfigParams{
		CertFile:      certFile,
		KeyFile:       keyFile,
		CAFile:        caFile,
		VerifyClients: verifyClients,
	})
}

// LoadClientTLSConfig 加载NATS客户端TLS配置
func LoadClientTLSConfig(certFile, keyFile, caFile string, insecureSkipVerify bool) (*tls.Config, error) {
	return LoadTLSConfigWithParams(&TLSConfigParams{
		CertFile:           certFile,
		KeyFile:            keyFile,
		CAFile:             caFile,
		InsecureSkipVerify: insecureSkipVerify,
	})
}
