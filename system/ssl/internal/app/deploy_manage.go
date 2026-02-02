package app

import (
	"context"
	"encoding/json"
	"time"
	"xiaozhizhang/system/ssl/internal/model"
)

// DeployCertificateToTargets 部署证书到多个目标
func (a *App) DeployCertificateToTargets(ctx context.Context, certificateID uint, targetIDs []uint, triggerType string) error {
	a.log.WithFields(map[string]interface{}{
		"certificate_id": certificateID,
		"target_ids":     targetIDs,
		"trigger_type":   triggerType,
	}).Info("开始部署证书")

	// 1. 获取证书
	cert, err := a.CertificateDao.FindById(ctx, certificateID)
	if err != nil {
		return a.err.New("获取证书失败", err)
	}

	// 2. 逐个部署到目标
	successCount := 0
	failCount := 0

	for _, targetID := range targetIDs {
		if err := a.deployCertificateToTarget(ctx, cert, targetID, triggerType); err != nil {
			a.log.WithErr(err).WithField("target_id", targetID).Error("部署证书失败")
			failCount++
		} else {
			successCount++
		}
	}

	a.log.WithFields(map[string]interface{}{
		"certificate_id": certificateID,
		"total":          len(targetIDs),
		"success":        successCount,
		"fail":           failCount,
	}).Info("证书部署完成")

	return nil
}

// deployCertificateToTarget 部署证书到单个目标
func (a *App) deployCertificateToTarget(ctx context.Context, cert *model.Certificate, targetID uint, triggerType string) error {
	startTime := time.Now()

	// 1. 获取部署目标
	target, err := a.DeployTargetDao.FindById(ctx, targetID)
	if err != nil {
		a.recordDeployHistory(ctx, uint(cert.ID), targetID, model.DeployStatusFailed, startTime, triggerType, err.Error(), "")
		return a.err.New("获取部署目标失败", err)
	}

	if target.Status != 1 {
		errMsg := "部署目标已禁用"
		a.recordDeployHistory(ctx, uint(cert.ID), targetID, model.DeployStatusFailed, startTime, triggerType, errMsg, "")
		return a.err.New(errMsg, nil).ValidWithCtx()
	}

	// 2. Resolve 部署配置（将引用式配置填充为运行时配置）
	runtimeTarget, err := a.resolveDeployTargetForRuntime(ctx, target)
	if err != nil {
		a.recordDeployHistory(ctx, uint(cert.ID), targetID, model.DeployStatusFailed, startTime, triggerType, err.Error(), "")
		return a.err.New("解析部署配置失败", err)
	}

	// 3. 调用部署服务
	resultData, err := a.DeployService.Deploy(ctx, runtimeTarget, cert.FullchainPem, cert.PrivkeyPem, cert.Domain)
	if err != nil {
		a.recordDeployHistory(ctx, uint(cert.ID), targetID, model.DeployStatusFailed, startTime, triggerType, err.Error(), "")
		return a.err.New("部署失败", err)
	}

	// 4. 记录部署成功
	a.recordDeployHistory(ctx, uint(cert.ID), targetID, model.DeployStatusSuccess, startTime, triggerType, "", resultData)

	return nil
}

// recordDeployHistory 记录部署历史
func (a *App) recordDeployHistory(ctx context.Context, certificateID, deployTargetID uint, status model.DeployStatus, startTime time.Time, triggerType, errorMessage, resultData string) {
	endTime := time.Now()

	// 处理 ResultData：空字符串转为 nil，避免 JSON 字段报错
	var resultDataPtr *string
	if resultData != "" {
		resultDataPtr = &resultData
	}

	history := &model.DeployHistory{
		CertificateID:  certificateID,
		DeployTargetID: deployTargetID,
		Status:         status,
		StartTime:      startTime,
		EndTime:        &endTime,
		ErrorMessage:   errorMessage,
		ResultData:     *resultDataPtr,
		TriggerType:    triggerType,
	}

	if err := a.DeployHistoryDao.Create(ctx, history); err != nil {
		a.log.WithErr(err).Error("记录部署历史失败")
	}
}

// GetDeployHistory 获取证书的部署历史
func (a *App) GetDeployHistory(ctx context.Context, certificateID uint, limit int) ([]model.DeployHistory, error) {
	return a.DeployHistoryDao.FindByCertificateID(ctx, certificateID, limit)
}

// resolveDeployTargetForRuntime 将引用式部署配置解析为运行时配置
// 根据部署类型从相关组件读取凭证信息，组装为 DeployService 所需的完整配置
func (a *App) resolveDeployTargetForRuntime(ctx context.Context, target *model.DeployTarget) (*model.DeployTarget, error) {
	// 创建临时 target 副本，避免修改原对象
	runtimeTarget := &model.DeployTarget{
		Model:       target.Model,
		Name:        target.Name,
		Domain:      target.Domain,
		Type:        target.Type,
		Status:      target.Status,
		Description: target.Description,
	}

	switch target.Type {
	case model.DeployTargetTypeLocal:
		// Local 类型不需要 resolve，直接复用
		runtimeTarget.Config = target.Config
		return runtimeTarget, nil

	case model.DeployTargetTypeSSH:
		return a.resolveSSHConfig(ctx, target)

	case model.DeployTargetTypeAliyunCAS:
		return a.resolveAliyunCASConfig(ctx, target)

	default:
		return nil, a.err.New("不支持的部署类型", nil).ValidWithCtx()
	}
}

// resolveSSHConfig 解析 SSH 部署配置
func (a *App) resolveSSHConfig(ctx context.Context, target *model.DeployTarget) (*model.DeployTarget, error) {
	// 1. 解析引用式配置
	var refConfig model.SSHDeployConfig
	if err := json.Unmarshal([]byte(target.Config), &refConfig); err != nil {
		return nil, a.err.New("解析SSH引用配置失败", err)
	}

	// 2. 从 server 组件获取 SSH 连接信息（已解密）
	sshConfig, err := a.serverFacade.GetServerSSHConfigByID(ctx, refConfig.ServerID)
	if err != nil {
		return nil, a.err.New("获取服务器SSH配置失败", err)
	}

	// 3. 组装运行时配置
	runtimeConfig := model.SSHDeployRuntimeConfig{
		Host:          sshConfig.Host,
		Port:          sshConfig.Port,
		Username:      sshConfig.Username,
		AuthMethod:    sshConfig.AuthMethod,
		Password:      sshConfig.Password,
		PrivateKey:    sshConfig.PrivateKey,
		RemotePath:    refConfig.RemotePath,
		FullchainName: refConfig.FullchainName,
		PrivkeyName:   refConfig.PrivkeyName,
		FileMode:      refConfig.FileMode,
		ReloadCommand: refConfig.ReloadCommand,
	}

	// 4. 序列化为 JSON
	configJSON, err := json.Marshal(runtimeConfig)
	if err != nil {
		return nil, a.err.New("序列化SSH运行时配置失败", err)
	}

	// 5. 创建运行时 target
	runtimeTarget := &model.DeployTarget{
		Model:       target.Model,
		Name:        target.Name,
		Domain:      target.Domain,
		Type:        target.Type,
		Config:      string(configJSON),
		Status:      target.Status,
		Description: target.Description,
	}

	return runtimeTarget, nil
}

// resolveAliyunCASConfig 解析阿里云 CAS 部署配置
func (a *App) resolveAliyunCASConfig(ctx context.Context, target *model.DeployTarget) (*model.DeployTarget, error) {
	// 1. 解析引用式配置
	var refConfig model.AliyunCASDeployConfig
	if err := json.Unmarshal([]byte(target.Config), &refConfig); err != nil {
		return nil, a.err.New("解析阿里云CAS引用配置失败", err)
	}

	// 2. 从 DnsCredential 获取 AK/SK（已解密）
	credential, err := a.DnsCredSvc.GetDecrypted(ctx, refConfig.DnsCredentialID)
	if err != nil {
		return nil, a.err.New("获取DNS凭证失败", err)
	}

	// 3. 组装运行时配置
	region := refConfig.Region
	if region == "" {
		// 尝试从 ExtraConfig 读取 region
		if credential.ExtraConfig != nil {
			if regionVal, ok := (*credential.ExtraConfig)["region"]; ok {
				if regionStr, ok := regionVal.(string); ok {
					region = regionStr
				}
			}
		}
		// 如果还是空，使用默认值（DeployService 会处理）
		if region == "" {
			region = "cn-hangzhou"
		}
	}

	runtimeConfig := model.AliyunCASDeployRuntimeConfig{
		AccessKeyID:     credential.AccessKey,
		AccessKeySecret: credential.SecretKey,
		Region:          region,
		AutoDeploy:      refConfig.AutoDeploy,
	}

	// 4. 序列化为 JSON
	configJSON, err := json.Marshal(runtimeConfig)
	if err != nil {
		return nil, a.err.New("序列化阿里云CAS运行时配置失败", err)
	}

	// 5. 创建运行时 target
	runtimeTarget := &model.DeployTarget{
		Model:       target.Model,
		Name:        target.Name,
		Domain:      target.Domain,
		Type:        target.Type,
		Config:      string(configJSON),
		Status:      target.Status,
		Description: target.Description,
	}

	return runtimeTarget, nil
}
