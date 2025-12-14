package app

import (
	"context"
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

	// 2. 调用部署服务
	resultData, err := a.DeployService.Deploy(ctx, target, cert.FullchainPem, cert.PrivkeyPem, cert.Domain)
	if err != nil {
		a.recordDeployHistory(ctx, uint(cert.ID), targetID, model.DeployStatusFailed, startTime, triggerType, err.Error(), "")
		return a.err.New("部署失败", err)
	}

	// 3. 记录部署成功
	a.recordDeployHistory(ctx, uint(cert.ID), targetID, model.DeployStatusSuccess, startTime, triggerType, "", resultData)

	return nil
}

// recordDeployHistory 记录部署历史
func (a *App) recordDeployHistory(ctx context.Context, certificateID, deployTargetID uint, status model.DeployStatus, startTime time.Time, triggerType, errorMessage, resultData string) {
	endTime := time.Now()
	history := &model.DeployHistory{
		CertificateID:  certificateID,
		DeployTargetID: deployTargetID,
		Status:         status,
		StartTime:      startTime,
		EndTime:        &endTime,
		ErrorMessage:   errorMessage,
		ResultData:     resultData,
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
