package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"xiaozhizhang/base"
	"xiaozhizhang/pkg/core/config"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/system/server/internal/model"
)

// EnsureBootstrapServerSSHCredentials 确保 bootstrap 服务器的 SSH 凭证已存在（启动时调用）
func (a *App) EnsureBootstrapServerSSHCredentials(ctx context.Context) error {
	// 从配置中读取 bootstrap 服务器列表
	bootstrapServers := base.Configures.Config.Server.Bootstrap
	if len(bootstrapServers) == 0 {
		a.log.Info("没有配置 bootstrap 服务器，跳过 SSH 凭证初始化")
		return nil
	}

	a.log.WithField("count", len(bootstrapServers)).Info("开始初始化 bootstrap 服务器 SSH 凭证")

	successCount := 0
	skipCount := 0

	// 逐个处理
	for _, bs := range bootstrapServers {
		// 若未配置 SSH，则跳过
		if bs.SSH == nil {
			a.log.WithField("server", bs.Name).Debug("未配置 SSH 凭证，跳过")
			continue
		}

		// 查询服务器是否存在
		server, err := a.ServerService.FindByName(ctx, bs.Name)
		if err != nil {
			if errorc.IsNotFound(err) {
				a.log.WithField("server", bs.Name).Warn("服务器不存在，跳过 SSH 凭证初始化（请先运行 EnsureBootstrapServers）")
				continue
			}
			a.log.WithErr(err).WithField("server", bs.Name).Error("查询服务器失败")
			return err
		}

		// 检查 SSH 凭证是否已存在
		exists, err := a.ServerSSHCredentialSvc.Exists(ctx, server.ID)
		if err != nil {
			a.log.WithErr(err).WithField("server", bs.Name).Error("检查 SSH 凭证是否存在失败")
			return err
		}

		if exists {
			a.log.WithField("server", bs.Name).Debug("SSH 凭证已存在，跳过")
			skipCount++
			continue
		}

		// 读取并校验配置
		sshConfig := bs.SSH
		if err := a.validateBootstrapSSHConfig(bs.Name, sshConfig); err != nil {
			a.log.WithErr(err).WithField("server", bs.Name).Error("SSH 配置校验失败")
			return err
		}

		// 读取私钥内容（优先从文件路径读取）
		privateKeyContent := sshConfig.PrivateKey
		if sshConfig.PrivateKeyFile != "" {
			content, err := os.ReadFile(sshConfig.PrivateKeyFile)
			if err != nil {
				a.log.WithErr(err).
					WithField("server", bs.Name).
					WithField("file", sshConfig.PrivateKeyFile).
					Error("读取私钥文件失败")
				return a.err().New("读取私钥文件失败", err)
			}
			privateKeyContent = string(content)
			a.log.WithField("server", bs.Name).
				WithField("file", sshConfig.PrivateKeyFile).
				Debug("从文件读取私钥成功")
		}

		// 设置默认端口
		port := sshConfig.Port
		if port == 0 {
			port = 22
		}

		// 构建凭证模型
		credential := &model.ServerSSHCredential{
			ServerID:   server.ID,
			Port:       port,
			Username:   sshConfig.Username,
			AuthMethod: sshConfig.AuthMethod,
			Password:   sshConfig.Password,
			PrivateKey: privateKeyContent,
			Comment:    sshConfig.Comment,
		}

		// 写入数据库（Service 会自动加密）
		if err := a.ServerSSHCredentialSvc.Upsert(ctx, credential); err != nil {
			a.log.WithErr(err).WithField("server", bs.Name).Error("创建 SSH 凭证失败")
			return err
		}

		a.log.WithField("server", bs.Name).
			WithField("username", sshConfig.Username).
			WithField("auth_method", sshConfig.AuthMethod).
			Info("bootstrap SSH 凭证初始化成功")
		successCount++
	}

	a.log.WithField("success", successCount).
		WithField("skip", skipCount).
		Info("bootstrap 服务器 SSH 凭证初始化完成")
	return nil
}

// validateBootstrapSSHConfig 校验 bootstrap SSH 配置
func (a *App) validateBootstrapSSHConfig(serverName string, sshConfig *config.BootstrapSSHCredential) error {
	if sshConfig.Username == "" {
		return a.err().New("SSH 用户名不能为空", nil).ValidWithCtx()
	}

	authMethod := strings.ToLower(sshConfig.AuthMethod)
	if authMethod != "password" && authMethod != "privatekey" {
		return a.err().New(
			fmt.Sprintf("SSH 认证方式必须是 password 或 privatekey，实际值：%s", sshConfig.AuthMethod),
			nil,
		).ValidWithCtx()
	}

	// 校验认证方式与字段一致性
	if authMethod == "password" {
		if sshConfig.Password == "" {
			return a.err().New("使用密码认证时必须提供 password", nil).ValidWithCtx()
		}
	}

	if authMethod == "privatekey" {
		if sshConfig.PrivateKey == "" && sshConfig.PrivateKeyFile == "" {
			return a.err().New("使用私钥认证时必须提供 private_key 或 private_key_file", nil).ValidWithCtx()
		}
	}

	return nil
}

// err 获取 ErrorBuilder（辅助方法）
func (a *App) err() *errorc.ErrorBuilder {
	return errorc.NewErrorBuilder("ServerApp")
}
