package service

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
)

// SSLService Agent 中的 SSL 证书部署服务
type SSLService struct {
	nginxSvc   *NginxService
	systemdSvc *SystemdService
	log        *logger.Log
	err        *errorc.ErrorBuilder
}

// NewSSLService 创建 SSL 服务
func NewSSLService(nginxSvc *NginxService, systemdSvc *SystemdService, log *logger.Log) *SSLService {
	return &SSLService{
		nginxSvc: nginxSvc,
		systemdSvc: systemdSvc,
		log:      log.WithEntryName("AgentSSLService"),
		err:      errorc.NewErrorBuilder("AgentSSLService"),
	}
}

// DeployCertificate 部署 SSL 证书到本机指定路径
func (s *SSLService) DeployCertificate(basePath, fullchainName, privkeyName, fullchainPem, privkeyPem, fileMode string) (fullchainPath, privkeyPath string, err error) {
	// 创建目录
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return "", "", s.err.New("创建目标目录失败", err)
	}

	// 确定文件名
	if fullchainName == "" {
		fullchainName = "fullchain.pem"
	}
	if privkeyName == "" {
		privkeyName = "privkey.pem"
	}

	// 确定文件权限
	mode := fs.FileMode(0600)
	if fileMode != "" {
		if parsed, err := parseFileModeOctal(fileMode); err == nil {
			mode = parsed
		}
	}

	// 写入证书文件
	fullchainPath = filepath.Join(basePath, fullchainName)
	privkeyPath = filepath.Join(basePath, privkeyName)

	if err := os.WriteFile(fullchainPath, []byte(fullchainPem), mode); err != nil {
		return "", "", s.err.New("写入 fullchain.pem 失败", err)
	}

	if err := os.WriteFile(privkeyPath, []byte(privkeyPem), mode); err != nil {
		return "", "", s.err.New("写入 privkey.pem 失败", err)
	}

	s.log.WithFields(map[string]interface{}{
		"fullchain_path": fullchainPath,
		"privkey_path":   privkeyPath,
	}).Info("SSL 证书部署成功")

	return fullchainPath, privkeyPath, nil
}

// ReloadService 受控服务重载（nginx 或 systemd）
func (s *SSLService) ReloadService(ctx context.Context, serviceType, serviceName string) (string, error) {
	switch serviceType {
	case "nginx":
		return s.nginxSvc.Reload(ctx)
	case "systemd":
		if serviceName == "" {
			return "", s.err.New("systemd 重载需要指定服务名", nil).ValidWithCtx()
		}
		return s.systemdSvc.ServiceControl(ctx, serviceName, "reload")
	default:
		return "", s.err.New(fmt.Sprintf("不支持的服务类型: %s", serviceType), nil).ValidWithCtx()
	}
}

func parseFileModeOctal(s string) (fs.FileMode, error) {
	var mode uint32
	_, err := fmt.Sscanf(s, "%o", &mode)
	if err != nil {
		// 尝试转为数字
		if num, err := strconv.ParseUint(s, 8, 32); err == nil {
			return fs.FileMode(num), nil
		}
		return 0, err
	}
	return fs.FileMode(mode), nil
}

