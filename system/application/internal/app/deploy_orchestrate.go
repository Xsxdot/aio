package app

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	nginxdto "xiaozhizhang/system/application/internal/model"
	"xiaozhizhang/system/application/internal/model/dto"
	registrydto "xiaozhizhang/system/registry/api/dto"
	systemddto "xiaozhizhang/system/systemd/api/dto"
)

// generateSystemdUnit 生成 systemd unit 文件内容
func generateSystemdUnit(serviceName, description, execStart, workingDir, user string) string {
	if user == "" {
		user = "root"
	}

	content := fmt.Sprintf(`[Unit]
Description=%s
After=network.target

[Service]
Type=simple
User=%s
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`, description, user, workingDir, execStart)

	return content
}

// Deploy 执行部署
func (a *App) Deploy(ctx context.Context, req *dto.DeployRequest) (*nginxdto.Deployment, error) {
	a.log.WithFields(map[string]interface{}{
		"applicationId": req.ApplicationID,
		"version":       req.Version,
	}).Info("开始部署")

	// 1. 获取应用信息
	application, err := a.ApplicationSvc.FindByID(ctx, req.ApplicationID)
	if err != nil {
		return nil, err
	}

	// 2. 序列化 spec 为 common.JSON
	var specMap map[string]interface{}
	if req.Spec != nil {
		specBytes, _ := json.Marshal(req.Spec)
		json.Unmarshal(specBytes, &specMap)
	}

	// 3. 创建 Release
	release := &nginxdto.Release{
		ApplicationID:      req.ApplicationID,
		Version:            req.Version,
		BackendArtifactID:  req.BackendArtifactID,
		FrontendArtifactID: req.FrontendArtifactID,
		Spec:               specMap,
		Status:             nginxdto.ReleaseStatusPending,
		Operator:           req.Operator,
	}
	if err := a.ReleaseSvc.Create(ctx, release); err != nil {
		return nil, err
	}

	// 4. 判断动作类型
	action := nginxdto.DeploymentActionDeploy
	if application.CurrentReleaseID > 0 {
		action = nginxdto.DeploymentActionUpdate
	}

	// 5. 创建 Deployment 记录
	deployment := &nginxdto.Deployment{
		ApplicationID: req.ApplicationID,
		ReleaseID:     release.ID,
		Action:        action,
		Status:        nginxdto.DeploymentStatusPending,
		Operator:      req.Operator,
	}
	if err := a.DeploymentSvc.Create(ctx, deployment); err != nil {
		return nil, err
	}

	// 6. 异步执行部署流程
	go a.executeDeployment(context.Background(), application, release, deployment, req.Spec)

	return deployment, nil
}

// Rollback 回滚到指定版本
func (a *App) Rollback(ctx context.Context, req *dto.RollbackRequest) (*nginxdto.Deployment, error) {
	a.log.WithFields(map[string]interface{}{
		"applicationId":   req.ApplicationID,
		"targetReleaseId": req.TargetReleaseID,
	}).Info("开始回滚")

	// 1. 获取应用信息
	application, err := a.ApplicationSvc.FindByID(ctx, req.ApplicationID)
	if err != nil {
		return nil, err
	}

	// 2. 获取目标版本
	targetRelease, err := a.ReleaseSvc.FindByID(ctx, req.TargetReleaseID)
	if err != nil {
		return nil, err
	}

	// 验证版本属于该应用
	if targetRelease.ApplicationID != req.ApplicationID {
		return nil, a.err.New("目标版本不属于该应用", nil).ValidWithCtx()
	}

	// 3. 创建回滚 Deployment 记录
	deployment := &nginxdto.Deployment{
		ApplicationID: req.ApplicationID,
		ReleaseID:     req.TargetReleaseID,
		Action:        nginxdto.DeploymentActionRollback,
		Status:        nginxdto.DeploymentStatusPending,
		Operator:      req.Operator,
	}
	if err := a.DeploymentSvc.Create(ctx, deployment); err != nil {
		return nil, err
	}

	// 4. 解析 spec
	var spec *dto.DeploySpec
	if targetRelease.Spec != nil {
		specBytes, _ := json.Marshal(targetRelease.Spec)
		json.Unmarshal(specBytes, &spec)
	}

	// 5. 异步执行回滚流程
	go a.executeDeployment(context.Background(), application, targetRelease, deployment, spec)

	return deployment, nil
}

// executeDeployment 执行部署流程（异步）
func (a *App) executeDeployment(ctx context.Context, application *nginxdto.Application, release *nginxdto.Release, deployment *nginxdto.Deployment, spec *dto.DeploySpec) {
	// 标记为运行中
	a.DeploymentSvc.MarkRunning(ctx, deployment.ID)
	a.ReleaseSvc.UpdateStatus(ctx, release.ID, nginxdto.ReleaseStatusDeploying)

	logStep := func(step string) {
		a.DeploymentSvc.AppendLog(ctx, deployment.ID, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), step))
	}

	defer func() {
		if r := recover(); r != nil {
			a.log.WithField("panic", r).Error("部署过程发生 panic")
			a.DeploymentSvc.MarkFailed(ctx, deployment.ID, fmt.Sprintf("panic: %v", r))
			a.ReleaseSvc.UpdateStatus(ctx, release.ID, nginxdto.ReleaseStatusFailed)
		}
	}()

	logStep("开始部署")

	// 1. 准备 release 目录
	releaseDir := filepath.Join(a.ReleaseDir, application.Project, application.Name, application.Env, release.Version)
	logStep(fmt.Sprintf("创建 release 目录: %s", releaseDir))

	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		a.failDeployment(ctx, deployment.ID, release.ID, fmt.Sprintf("创建 release 目录失败: %v", err))
		return
	}

	// 2. 下载并解压产物
	if release.BackendArtifactID > 0 {
		logStep("下载并解压后端产物...")
		if err := a.extractArtifact(ctx, release.BackendArtifactID, releaseDir); err != nil {
			a.failDeployment(ctx, deployment.ID, release.ID, fmt.Sprintf("解压后端产物失败: %v", err))
			return
		}
		logStep("后端产物解压完成")
	}

	if release.FrontendArtifactID > 0 {
		logStep("下载并解压前端产物...")
		webDir := filepath.Join(releaseDir, "web")
		if err := os.MkdirAll(webDir, 0755); err != nil {
			a.failDeployment(ctx, deployment.ID, release.ID, fmt.Sprintf("创建 web 目录失败: %v", err))
			return
		}
		if err := a.extractArtifact(ctx, release.FrontendArtifactID, webDir); err != nil {
			a.failDeployment(ctx, deployment.ID, release.ID, fmt.Sprintf("解压前端产物失败: %v", err))
			return
		}
		logStep("前端产物解压完成")
	}

	// 更新 release 路径
	a.ReleaseSvc.UpdateReleasePath(ctx, release.ID, releaseDir)

	// 3. 部署 systemd（后端/fullstack）
	if application.Type == nginxdto.ApplicationTypeBackend || application.Type == nginxdto.ApplicationTypeFullstack {
		logStep("配置 systemd 服务...")
		if err := a.deploySystemd(ctx, application, release, releaseDir, spec); err != nil {
			a.failDeployment(ctx, deployment.ID, release.ID, fmt.Sprintf("配置 systemd 失败: %v", err))
			return
		}
		logStep("systemd 服务配置完成")
	}

	// 4. 部署 nginx
	logStep("配置 nginx...")
	if err := a.deployNginx(ctx, application, release, releaseDir, spec); err != nil {
		a.failDeployment(ctx, deployment.ID, release.ID, fmt.Sprintf("配置 nginx 失败: %v", err))
		return
	}
	logStep("nginx 配置完成")

	// 5. 注册到 registry（后端/fullstack）
	if application.Type == nginxdto.ApplicationTypeBackend || application.Type == nginxdto.ApplicationTypeFullstack {
		logStep("注册到服务注册中心...")
		if err := a.registerToRegistry(ctx, application, release); err != nil {
			// 注册失败不阻塞部署，只记录警告
			logStep(fmt.Sprintf("注册服务失败（非致命）: %v", err))
		} else {
			logStep("服务注册完成")
		}
	}

	// 6. 标记为成功
	a.DeploymentSvc.MarkSuccess(ctx, deployment.ID)
	a.ReleaseSvc.MarkAsActive(ctx, application.ID, release.ID)
	a.ApplicationSvc.UpdateCurrentRelease(ctx, application.ID, release.ID)

	logStep("部署完成")
}

// failDeployment 标记部署失败
func (a *App) failDeployment(ctx context.Context, deploymentID, releaseID int64, errMsg string) {
	a.DeploymentSvc.MarkFailed(ctx, deploymentID, errMsg)
	a.ReleaseSvc.UpdateStatus(ctx, releaseID, nginxdto.ReleaseStatusFailed)
}

// extractArtifact 下载并解压产物
func (a *App) extractArtifact(ctx context.Context, artifactID int64, targetDir string) error {
	// 获取产物信息
	artifact, err := a.ArtifactSvc.FindByID(ctx, artifactID)
	if err != nil {
		return err
	}

	// 从存储下载
	reader, err := a.Storage.Open(ctx, artifact.ObjectKey)
	if err != nil {
		return fmt.Errorf("打开产物失败: %w", err)
	}
	defer reader.Close()

	// 根据文件类型解压
	if strings.HasSuffix(artifact.FileName, ".tar.gz") || strings.HasSuffix(artifact.FileName, ".tgz") {
		return a.extractTarGz(reader, targetDir)
	}

	// 单文件直接复制
	targetPath := filepath.Join(targetDir, artifact.FileName)
	outFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, reader); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 设置可执行权限（如果是二进制文件）
	if artifact.Type == nginxdto.ArtifactTypeBackend {
		os.Chmod(targetPath, 0755)
	}

	return nil
}

// extractTarGz 解压 tar.gz 文件
func (a *App) extractTarGz(reader io.Reader, targetDir string) error {
	gzr, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("创建 gzip reader 失败: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar header 失败: %w", err)
		}

		// 安全检查：防止目录穿越
		target := filepath.Join(targetDir, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(targetDir)+string(os.PathSeparator)) {
			return fmt.Errorf("非法路径: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("创建目录失败: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("创建父目录失败: %w", err)
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("创建文件失败: %w", err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("写入文件失败: %w", err)
			}
			outFile.Close()
		}
	}

	return nil
}

// deploySystemd 部署 systemd 服务
func (a *App) deploySystemd(ctx context.Context, app *nginxdto.Application, release *nginxdto.Release, releaseDir string, spec *dto.DeploySpec) error {
	serviceName := fmt.Sprintf("%s-%s-%s.service", app.Project, app.Name, app.Env)

	// 构建启动命令
	startCommand := ""
	workingDir := releaseDir
	if spec != nil && spec.Backend != nil {
		startCommand = spec.Backend.StartCommand
		if spec.Backend.WorkingDir != "" {
			workingDir = spec.Backend.WorkingDir
		}
	}

	// 替换变量
	startCommand = strings.ReplaceAll(startCommand, "${installPath}", releaseDir)
	startCommand = strings.ReplaceAll(startCommand, "${env}", app.Env)
	startCommand = strings.ReplaceAll(startCommand, "${version}", release.Version)

	// 如果没有指定启动命令，尝试使用默认命令
	if startCommand == "" {
		// 查找可执行文件
		execPath := filepath.Join(releaseDir, app.Name)
		if _, err := os.Stat(execPath); err == nil {
			startCommand = execPath
		}
	}

	if startCommand == "" {
		return fmt.Errorf("未指定启动命令")
	}

	// 生成 unit 文件内容
	description := fmt.Sprintf("%s %s (%s)", app.Project, app.Name, app.Env)
	unitContent := generateSystemdUnit(serviceName, description, startCommand, workingDir, "root")

	// 调用 systemd client 创建服务
	req := &systemddto.CreateServiceReq{
		Name:    serviceName,
		Content: unitContent,
	}

	err := a.SystemdModule.Client.CreateService(ctx, req)
	if err != nil {
		return err
	}

	// 启动并设为开机自启
	if err := a.SystemdModule.Client.SetServiceEnabled(ctx, serviceName, true); err != nil {
		a.log.WithErr(err).Warn("设置服务开机自启失败")
	}

	if err := a.SystemdModule.Client.ControlService(ctx, serviceName, "restart"); err != nil {
		return fmt.Errorf("启动服务失败: %w", err)
	}

	return nil
}

// deployNginx 部署 nginx 配置
func (a *App) deployNginx(ctx context.Context, app *nginxdto.Application, release *nginxdto.Release, releaseDir string, spec *dto.DeploySpec) error {
	// 生成 nginx 配置内容
	configContent := a.generateNginxConfig(app, releaseDir, spec)

	// 配置文件名
	configFileName := fmt.Sprintf("%s-%s-%s.conf", app.Project, app.Name, app.Env)

	// 调用 nginx client 创建配置
	// 这里需要确保有本地 target
	// TODO: 实际实现需要根据现有的 nginx target 创建或更新配置

	a.log.WithFields(map[string]interface{}{
		"configFileName": configFileName,
		"domain":         app.Domain,
		"content":        configContent[:min(200, len(configContent))] + "...",
	}).Info("生成 nginx 配置")

	return nil
}

// generateNginxConfig 生成 nginx 配置内容
func (a *App) generateNginxConfig(app *nginxdto.Application, releaseDir string, spec *dto.DeploySpec) string {
	var sb strings.Builder

	sb.WriteString("server {\n")

	// 监听端口
	if app.SSL {
		sb.WriteString("    listen 443 ssl http2;\n")
		sb.WriteString("    listen [::]:443 ssl http2;\n")
	} else {
		sb.WriteString("    listen 80;\n")
		sb.WriteString("    listen [::]:80;\n")
	}

	// 域名
	if app.Domain != "" {
		sb.WriteString(fmt.Sprintf("    server_name %s;\n", app.Domain))
	}

	// SSL 配置
	if app.SSL {
		certPath := "/etc/ssl/certs/" + app.Domain + ".crt"
		keyPath := "/etc/ssl/private/" + app.Domain + ".key"
		if spec != nil && spec.SSLCertPath != "" {
			certPath = spec.SSLCertPath
		}
		if spec != nil && spec.SSLKeyPath != "" {
			keyPath = spec.SSLKeyPath
		}
		sb.WriteString(fmt.Sprintf("    ssl_certificate %s;\n", certPath))
		sb.WriteString(fmt.Sprintf("    ssl_certificate_key %s;\n", keyPath))
		sb.WriteString("    ssl_protocols TLSv1.2 TLSv1.3;\n")
		sb.WriteString("    ssl_prefer_server_ciphers on;\n")
	}

	// 根据应用类型配置
	switch app.Type {
	case nginxdto.ApplicationTypeFrontend:
		// 静态站点
		rootPath := filepath.Join(releaseDir, "web")
		if spec != nil && spec.Frontend != nil && spec.Frontend.RootPath != "" {
			rootPath = spec.Frontend.RootPath
		}
		sb.WriteString(fmt.Sprintf("    root %s;\n", rootPath))
		sb.WriteString("    index index.html;\n")
		sb.WriteString("    location / {\n")
		sb.WriteString("        try_files $uri $uri/ /index.html;\n")
		sb.WriteString("    }\n")

	case nginxdto.ApplicationTypeBackend:
		// 反向代理
		port := app.Port
		if port == 0 && spec != nil && spec.Backend != nil {
			port = spec.Backend.Port
		}
		if port == 0 {
			port = 8080
		}
		sb.WriteString("    location / {\n")
		sb.WriteString(fmt.Sprintf("        proxy_pass http://127.0.0.1:%d;\n", port))
		sb.WriteString("        proxy_set_header Host $host;\n")
		sb.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
		sb.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		sb.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
		sb.WriteString("    }\n")

	case nginxdto.ApplicationTypeFullstack:
		// 前后端混合
		rootPath := filepath.Join(releaseDir, "web")
		if spec != nil && spec.Frontend != nil && spec.Frontend.RootPath != "" {
			rootPath = spec.Frontend.RootPath
		}
		sb.WriteString(fmt.Sprintf("    root %s;\n", rootPath))
		sb.WriteString("    index index.html;\n")

		// API 代理
		port := app.Port
		if port == 0 && spec != nil && spec.Backend != nil {
			port = spec.Backend.Port
		}
		if port == 0 {
			port = 8080
		}
		sb.WriteString("    location /api/ {\n")
		sb.WriteString(fmt.Sprintf("        proxy_pass http://127.0.0.1:%d;\n", port))
		sb.WriteString("        proxy_set_header Host $host;\n")
		sb.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
		sb.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		sb.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
		sb.WriteString("    }\n")

		sb.WriteString("    location / {\n")
		sb.WriteString("        try_files $uri $uri/ /index.html;\n")
		sb.WriteString("    }\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// registerToRegistry 注册到服务中心
func (a *App) registerToRegistry(ctx context.Context, app *nginxdto.Application, release *nginxdto.Release) error {
	// 确保服务定义存在（不再需要 env）
	svcReq := &registrydto.CreateServiceReq{
		Project:     app.Project,
		Name:        app.Name,
		Owner:       app.Owner,
		Description: app.Description,
	}

	svc, _, err := a.RegistryModule.Client.EnsureService(ctx, svcReq)
	if err != nil {
		return err
	}

	// 更新应用的 registry service id
	a.ApplicationSvc.Update(ctx, app.ID, &nginxdto.Application{RegistryServiceID: svc.ID})

	// 注册实例（env 在实例级别）
	endpoint := fmt.Sprintf("http://%s:%d", "127.0.0.1", app.Port)
	if app.SSL && app.Domain != "" {
		endpoint = fmt.Sprintf("https://%s", app.Domain)
	}

	instanceReq := &registrydto.RegisterInstanceReq{
		ServiceID:   svc.ID,
		InstanceKey: fmt.Sprintf("%s-%s", app.Name, app.Env),
		Env:         app.Env,
		Host:        "127.0.0.1",
		Endpoint:    endpoint,
		Meta: map[string]interface{}{
			"version":    release.Version,
			"releaseId":  release.ID,
			"deployedAt": time.Now().Format(time.RFC3339),
		},
		TTLSeconds: 300, // 5 分钟 TTL
	}

	_, err = a.RegistryModule.Client.RegisterInstance(ctx, instanceReq)
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
