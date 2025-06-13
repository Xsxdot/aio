package certmanager

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// ================ 具体部署实现 ================

// deployToLocal 部署证书到本地文件
func (cm *CertManager) deployToLocal(ctx context.Context, cert *DomainCert, config *LocalConfig) error {
	cm.logger.Info("开始本地文件部署",
		zap.String("domain", cert.Domain),
		zap.String("cert_path", config.CertPath),
		zap.String("key_path", config.KeyPath))

	// 获取证书内容
	var certContent, keyContent []byte

	// 优先使用存储在etcd中的证书内容
	if cert.CertContent != "" && cert.KeyContent != "" {
		certContent = []byte(cert.CertContent)
		keyContent = []byte(cert.KeyContent)
	} else if cert.CertPath != "" && cert.KeyPath != "" {
		// 兼容性处理：从文件读取
		var err error
		certContent, err = os.ReadFile(cert.CertPath)
		if err != nil {
			return fmt.Errorf("读取源证书文件失败: %v", err)
		}

		keyContent, err = os.ReadFile(cert.KeyPath)
		if err != nil {
			return fmt.Errorf("读取源私钥文件失败: %v", err)
		}
	} else {
		return fmt.Errorf("域名 %s 没有可用的证书内容", cert.Domain)
	}

	// 确保目标目录存在
	certDir := filepath.Dir(config.CertPath)
	if err := createDirIfNotExist(certDir); err != nil {
		return fmt.Errorf("创建证书目录失败: %v", err)
	}

	keyDir := filepath.Dir(config.KeyPath)
	if err := createDirIfNotExist(keyDir); err != nil {
		return fmt.Errorf("创建私钥目录失败: %v", err)
	}

	// 复制证书文件
	if err := os.WriteFile(config.CertPath, certContent, 0644); err != nil {
		return fmt.Errorf("写入证书文件失败: %v", err)
	}

	// 复制私钥文件
	if err := os.WriteFile(config.KeyPath, keyContent, 0600); err != nil {
		return fmt.Errorf("写入私钥文件失败: %v", err)
	}

	cm.logger.Info("本地文件部署完成",
		zap.String("domain", cert.Domain),
		zap.String("cert_path", config.CertPath),
		zap.String("key_path", config.KeyPath))

	// 执行部署后命令
	if len(config.PostDeployCommands) > 0 {
		cm.logger.Info("开始执行本地部署后命令",
			zap.String("domain", cert.Domain),
			zap.Int("command_count", len(config.PostDeployCommands)))

		if err := cm.executeLocalCommands(ctx, config.PostDeployCommands); err != nil {
			cm.logger.Error("执行本地部署后命令失败",
				zap.String("domain", cert.Domain),
				zap.Error(err))
			return fmt.Errorf("执行部署后命令失败: %v", err)
		}

		cm.logger.Info("本地部署后命令执行完成",
			zap.String("domain", cert.Domain))
	}

	return nil
}

// deployToRemote 部署证书到远程服务器
func (cm *CertManager) deployToRemote(ctx context.Context, cert *DomainCert, config *RemoteConfig) error {
	cm.logger.Info("开始远程服务器部署",
		zap.String("domain", cert.Domain),
		zap.String("host", config.Host),
		zap.Int("port", config.Port),
		zap.String("username", config.Username))

	// 获取证书内容
	var certContent, keyContent []byte

	// 优先使用存储在etcd中的证书内容
	if cert.CertContent != "" && cert.KeyContent != "" {
		certContent = []byte(cert.CertContent)
		keyContent = []byte(cert.KeyContent)
	} else if cert.CertPath != "" && cert.KeyPath != "" {
		// 兼容性处理：从文件读取
		var err error
		certContent, err = os.ReadFile(cert.CertPath)
		if err != nil {
			return fmt.Errorf("读取源证书文件失败: %v", err)
		}

		keyContent, err = os.ReadFile(cert.KeyPath)
		if err != nil {
			return fmt.Errorf("读取源私钥文件失败: %v", err)
		}
	} else {
		return fmt.Errorf("域名 %s 没有可用的证书内容", cert.Domain)
	}

	// 建立SSH连接
	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 在生产环境中应该使用更安全的验证
		Timeout:         30 * time.Second,
	}

	// 设置认证方式
	if config.PrivateKey != "" {
		// 使用私钥认证
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return fmt.Errorf("解析SSH私钥失败: %v", err)
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else {
		// 使用密码认证
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(config.Password)}
	}

	// 连接到远程服务器
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("连接SSH服务器失败: %v", err)
	}
	defer client.Close()

	// 创建远程目录并上传证书文件
	if err := cm.uploadFileViaSSH(client, certContent, config.CertPath); err != nil {
		return fmt.Errorf("上传证书文件失败: %v", err)
	}

	if err := cm.uploadFileViaSSH(client, keyContent, config.KeyPath); err != nil {
		return fmt.Errorf("上传私钥文件失败: %v", err)
	}

	cm.logger.Info("远程服务器部署完成",
		zap.String("domain", cert.Domain),
		zap.String("host", config.Host))

	// 执行部署后命令
	if len(config.PostDeployCommands) > 0 {
		cm.logger.Info("开始执行远程部署后命令",
			zap.String("domain", cert.Domain),
			zap.String("host", config.Host),
			zap.Int("command_count", len(config.PostDeployCommands)))

		if err := cm.executeRemoteCommands(ctx, client, config.PostDeployCommands); err != nil {
			cm.logger.Error("执行远程部署后命令失败",
				zap.String("domain", cert.Domain),
				zap.String("host", config.Host),
				zap.Error(err))
			return fmt.Errorf("执行部署后命令失败: %v", err)
		}

		cm.logger.Info("远程部署后命令执行完成",
			zap.String("domain", cert.Domain),
			zap.String("host", config.Host))
	}

	return nil
}

// uploadFileViaSSH 通过SSH上传文件
func (cm *CertManager) uploadFileViaSSH(client *ssh.Client, content []byte, remotePath string) error {
	// 创建远程目录
	remoteDir := filepath.Dir(remotePath)
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	// 确保远程目录存在
	mkdirCmd := fmt.Sprintf("mkdir -p %s", remoteDir)
	if err := session.Run(mkdirCmd); err != nil {
		cm.logger.Warn("创建远程目录失败（可能已存在）", zap.String("dir", remoteDir), zap.Error(err))
	}

	// 创建新的会话来上传文件
	session, err = client.NewSession()
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	// 使用SCP协议上传文件
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()

		// SCP协议格式
		fmt.Fprintf(w, "C0644 %d %s\n", len(content), filepath.Base(remotePath))
		w.Write(content)
		fmt.Fprint(w, "\x00")
	}()

	scpCmd := fmt.Sprintf("scp -t %s", remotePath)
	if err := session.Run(scpCmd); err != nil {
		return fmt.Errorf("SCP上传失败: %v", err)
	}

	return nil
}

// deployToAliyunCDN 部署证书到阿里云CDN
func (cm *CertManager) deployToAliyunCDN(ctx context.Context, cert *DomainCert, config *AliyunConfig) error {
	// 调用真实的阿里云CDN部署实现
	return cm.deployToAliyunCDNReal(ctx, cert, config)
}

// TestSSHConnection 测试SSH连接
func (cm *CertManager) TestSSHConnection(config *RemoteConfig) error {
	sshConfig := &ssh.ClientConfig{
		User:            config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 设置认证方式
	if config.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(config.PrivateKey))
		if err != nil {
			return fmt.Errorf("解析SSH私钥失败: %v", err)
		}
		sshConfig.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	} else {
		sshConfig.Auth = []ssh.AuthMethod{ssh.Password(config.Password)}
	}

	// 连接到远程服务器
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("连接SSH服务器失败: %v", err)
	}
	defer client.Close()

	// 执行简单的测试命令
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建SSH会话失败: %v", err)
	}
	defer session.Close()

	if err := session.Run("echo 'test'"); err != nil {
		return fmt.Errorf("执行测试命令失败: %v", err)
	}

	return nil
}

// TestTLSConnection 测试TLS连接
func (cm *CertManager) TestTLSConnection(host string, port int) error {
	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", host, port), &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return fmt.Errorf("TLS连接失败: %v", err)
	}
	defer conn.Close()

	return nil
}

// TestTCPConnection 测试TCP连接
func (cm *CertManager) TestTCPConnection(host string, port int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 10*time.Second)
	if err != nil {
		return fmt.Errorf("TCP连接失败: %v", err)
	}
	defer conn.Close()

	return nil
}

// executeLocalCommands 执行本地命令
func (cm *CertManager) executeLocalCommands(ctx context.Context, commands []string) error {
	for i, cmd := range commands {
		if cmd == "" {
			continue
		}

		// 增强命令，处理路径问题
		enhancedCmd := enhanceCommand(cmd)

		cm.logger.Info("执行本地命令",
			zap.Int("index", i+1),
			zap.String("original_command", cmd),
			zap.String("enhanced_command", enhancedCmd))

		// 使用登录shell来确保环境变量被正确加载
		execCmd := exec.CommandContext(ctx, "bash", "-l", "-c", enhancedCmd)

		// 设置常用的环境变量路径
		execCmd.Env = append(os.Environ(),
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		)

		output, err := execCmd.CombinedOutput()

		if err != nil {
			cm.logger.Error("本地命令执行失败",
				zap.Int("index", i+1),
				zap.String("original_command", cmd),
				zap.String("enhanced_command", enhancedCmd),
				zap.String("output", string(output)),
				zap.Error(err))
			return fmt.Errorf("命令 %d 执行失败: %s, 输出: %s", i+1, err.Error(), string(output))
		}

		cm.logger.Info("本地命令执行成功",
			zap.Int("index", i+1),
			zap.String("command", enhancedCmd),
			zap.String("output", string(output)))
	}

	return nil
}

// executeRemoteCommands 执行远程命令
func (cm *CertManager) executeRemoteCommands(ctx context.Context, client *ssh.Client, commands []string) error {
	for i, cmd := range commands {
		if cmd == "" {
			continue
		}

		cm.logger.Info("执行远程命令",
			zap.Int("index", i+1),
			zap.String("command", cmd))

		// 创建新的SSH会话
		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("创建SSH会话失败: %v", err)
		}

		// 构建增强的shell命令，确保环境变量和路径正确
		// 使用多种方式来确保命令能被找到
		shellCmd := fmt.Sprintf(`
			# 设置完整的PATH环境变量
			export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
			
			# 加载用户环境配置
			source /etc/profile 2>/dev/null || true
			source ~/.profile 2>/dev/null || true
			source ~/.bashrc 2>/dev/null || true
			
			# 执行命令
			%s
		`, cmd)

		// 执行命令并获取输出
		output, err := session.CombinedOutput(shellCmd)
		session.Close()

		if err != nil {
			cm.logger.Error("远程命令执行失败",
				zap.Int("index", i+1),
				zap.String("command", cmd),
				zap.String("output", string(output)),
				zap.Error(err))
			return fmt.Errorf("命令 %d 执行失败: %s, 输出: %s", i+1, err.Error(), string(output))
		}

		cm.logger.Info("远程命令执行成功",
			zap.Int("index", i+1),
			zap.String("command", cmd),
			zap.String("output", string(output)))
	}

	return nil
}

// findCommandPath 查找命令的完整路径
func findCommandPath(cmdName string) string {
	// 常见的系统路径
	paths := []string{
		"/usr/sbin/" + cmdName,
		"/usr/bin/" + cmdName,
		"/sbin/" + cmdName,
		"/bin/" + cmdName,
		"/usr/local/sbin/" + cmdName,
		"/usr/local/bin/" + cmdName,
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return cmdName // 如果找不到，返回原始命令名
}

// enhanceCommand 增强命令，处理常见的路径问题
func enhanceCommand(cmd string) string {
	// 常见的需要完整路径的命令
	commonCommands := map[string]bool{
		"nginx":         true,
		"apache2":       true,
		"httpd":         true,
		"systemctl":     true,
		"service":       true,
		"docker":        true,
		"supervisorctl": true,
	}

	// 简单的命令解析 - 获取第一个词（命令名）
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return cmd
	}

	cmdName := parts[0]
	if commonCommands[cmdName] {
		fullPath := findCommandPath(cmdName)
		if fullPath != cmdName {
			// 替换命令名为完整路径
			parts[0] = fullPath
			return strings.Join(parts, " ")
		}
	}

	return cmd
}
