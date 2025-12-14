package app

import (
	"context"
	"time"
	"xiaozhizhang/base"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/ssl/internal/model"
	"xiaozhizhang/system/ssl/internal/service"
)

// IssueCertificateRequest 证书签发请求
type IssueCertificateRequest struct {
	Name            string `json:"name"`
	Domain          string `json:"domain"`
	Email           string `json:"email"`
	DnsCredentialID uint   `json:"dns_credential_id"`
	RenewBeforeDays int    `json:"renew_before_days"`
	AutoRenew       bool   `json:"auto_renew"`
	AutoDeploy      bool   `json:"auto_deploy"` // 如果为 true，会根据证书域名自动匹配部署目标
	Description     string `json:"description"`
	UseStaging      bool   `json:"use_staging"` // 是否使用测试环境
}

// IssueCertificate 签发证书
// 完整流程：验证 -> 签发 -> 落库 -> 触发部署
func (a *App) IssueCertificate(ctx context.Context, req *IssueCertificateRequest) (*model.Certificate, error) {
	a.log.WithFields(map[string]interface{}{
		"name":   req.Name,
		"domain": req.Domain,
	}).Info("开始签发证书")

	// 1. 获取 DNS 凭证（解密）
	dnsCredential, err := a.DnsCredSvc.GetDecrypted(ctx, req.DnsCredentialID)
	if err != nil {
		return nil, a.err.New("获取 DNS 凭证失败", err)
	}

	if dnsCredential.Status != 1 {
		return nil, a.err.New("DNS 凭证已禁用", nil).ValidWithCtx()
	}

	// 2. 构造 ACME 请求
	acmeReq := &service.IssueCertificateRequest{
		Domains:    []string{req.Domain},
		Email:      req.Email,
		Provider:   dnsCredential.Provider,
		AccessKey:  dnsCredential.AccessKey,
		SecretKey:  dnsCredential.SecretKey,
		UseStaging: req.UseStaging,
	}

	// 3. 调用 ACME 服务签发证书
	acmeResp, err := a.AcmeService.IssueCertificate(acmeReq)
	if err != nil {
		return nil, a.err.New("ACME 签发证书失败", err)
	}

	// 4. 加密账户私钥
	encryptedAccountKey, err := a.CryptoService.Encrypt(acmeResp.AccountKeyPem)
	if err != nil {
		return nil, a.err.New("加密账户私钥失败", err)
	}

	// 5. 构造证书记录
	autoRenew := 0
	if req.AutoRenew {
		autoRenew = 1
	}
	autoDeploy := 0
	if req.AutoDeploy {
		autoDeploy = 1
	}

	renewBeforeDays := req.RenewBeforeDays
	if renewBeforeDays <= 0 {
		renewBeforeDays = 30 // 默认提前 30 天续期
	}

	issuedAt := acmeResp.IssuedAt
	certificate := &model.Certificate{
		Name:            req.Name,
		Domain:          req.Domain,
		Email:           req.Email,
		DnsCredentialID: req.DnsCredentialID,
		Status:          model.CertificateStatusActive,
		ExpiresAt:       &acmeResp.ExpiresAt,
		IssuedAt:        &issuedAt,
		RenewBeforeDays: renewBeforeDays,
		FullchainPem:    acmeResp.FullchainPem,
		PrivkeyPem:      acmeResp.PrivkeyPem,
		AcmeAccountURL:  acmeResp.AccountURL,
		AcmeAccountKey:  encryptedAccountKey,
		CertURL:         acmeResp.CertURL,
		AutoRenew:       autoRenew,
		AutoDeploy:      autoDeploy,
		Description:     req.Description,
	}

	// 6. 保存证书到数据库
	if err := a.CertificateDao.Create(ctx, certificate); err != nil {
		return nil, a.err.New("保存证书失败", err)
	}

	a.log.WithField("certificate_id", certificate.ID).Info("证书签发成功")

	// 7. 触发自动部署（根据证书域名自动匹配部署目标）
	if req.AutoDeploy {
		go func() {
			deployCtx := context.Background()
			// 按证书域名自动匹配部署目标
			targetIDs, err := a.DeployTargetSvc.MatchTargetsByCertificateDomain(deployCtx, certificate.Domain)
			if err != nil {
				a.log.WithErr(err).WithField("certificate_id", certificate.ID).Error("匹配部署目标失败")
				return
			}

			if len(targetIDs) > 0 {
				if err := a.DeployCertificateToTargets(deployCtx, uint(certificate.ID), targetIDs, "auto_issue"); err != nil {
					a.log.WithErr(err).WithField("certificate_id", certificate.ID).Error("自动部署证书失败")
				}
			} else {
				a.log.WithField("certificate_id", certificate.ID).Info("未找到匹配的部署目标，跳过自动部署")
			}
		}()
	}

	return certificate, nil
}

// RenewCertificate 续期证书
func (a *App) RenewCertificate(ctx context.Context, certificateID uint) error {
	a.log.WithField("certificate_id", certificateID).Info("开始续期证书")

	// 1. 获取证书记录
	cert, err := a.CertificateDao.FindById(ctx, certificateID)
	if err != nil {
		return a.err.New("获取证书记录失败", err)
	}

	// 2. 更新状态为续期中
	if err := a.CertificateDao.UpdateStatus(ctx, certificateID, model.CertificateStatusRenewing); err != nil {
		return a.err.New("更新证书状态失败", err)
	}

	// 3. 获取 DNS 凭证（解密）
	dnsCredential, err := a.DnsCredSvc.GetDecrypted(ctx, cert.DnsCredentialID)
	if err != nil {
		a.updateCertError(ctx, certificateID, "获取 DNS 凭证失败: "+err.Error())
		return a.err.New("获取 DNS 凭证失败", err)
	}

	// 4. 解密账户私钥
	accountKey, err := a.CryptoService.Decrypt(cert.AcmeAccountKey)
	if err != nil {
		a.updateCertError(ctx, certificateID, "解密账户私钥失败: "+err.Error())
		return a.err.New("解密账户私钥失败", err)
	}

	// 5. 构造 ACME 续期请求
	acmeReq := &service.IssueCertificateRequest{
		Domains:    []string{cert.Domain},
		Email:      cert.Email,
		Provider:   dnsCredential.Provider,
		AccessKey:  dnsCredential.AccessKey,
		SecretKey:  dnsCredential.SecretKey,
		AccountKey: accountKey,
		UseStaging: false,
	}

	// 6. 调用 ACME 服务续期证书
	acmeResp, err := a.AcmeService.RenewCertificate(acmeReq, cert.FullchainPem, cert.PrivkeyPem)
	if err != nil {
		a.updateCertError(ctx, certificateID, "ACME 续期失败: "+err.Error())
		return a.err.New("ACME 续期证书失败", err)
	}

	// 7. 更新证书记录
	now := time.Now()
	cert.FullchainPem = acmeResp.FullchainPem
	cert.PrivkeyPem = acmeResp.PrivkeyPem
	cert.ExpiresAt = &acmeResp.ExpiresAt
	cert.LastRenewAt = &now
	cert.CertURL = acmeResp.CertURL
	cert.Status = model.CertificateStatusActive
	cert.LastError = ""

	if _, err := a.CertificateDao.UpdateById(ctx, certificateID, cert); err != nil {
		return a.err.New("更新证书记录失败", err)
	}

	a.log.WithField("certificate_id", certificateID).Info("证书续期成功")

	// 8. 触发自动部署（按证书域名匹配部署目标）
	if cert.AutoDeploy == 1 {
		go func() {
			deployCtx := context.Background()
			// 按证书域名匹配部署目标
			targetIDs, err := a.DeployTargetSvc.MatchTargetsByCertificateDomain(deployCtx, cert.Domain)
			if err != nil {
				a.log.WithErr(err).WithField("certificate_id", certificateID).Error("匹配部署目标失败")
				return
			}

			if len(targetIDs) > 0 {
				if err := a.DeployCertificateToTargets(deployCtx, uint(certificateID), targetIDs, "auto_renew"); err != nil {
					a.log.WithErr(err).WithField("certificate_id", certificateID).Error("自动部署证书失败")
				}
			} else {
				a.log.WithField("certificate_id", certificateID).Info("未找到匹配的部署目标，跳过自动部署")
			}
		}()
	}

	return nil
}

// RenewDueCertificates 扫描并续期即将过期的证书
// 被 scheduler 定时任务调用
func (a *App) RenewDueCertificates(ctx context.Context) error {
	a.log.Info("开始扫描需要续期的证书")

	// 1. 查询需要续期的证书
	certs, err := a.CertificateDao.FindCertificatesToRenew(ctx)
	if err != nil {
		return a.err.New("查询需要续期的证书失败", err)
	}

	a.log.WithField("count", len(certs)).Info("找到需要续期的证书")

	// 2. 逐个续期
	successCount := 0
	failCount := 0

	for _, cert := range certs {
		if err := a.RenewCertificate(ctx, uint(cert.ID)); err != nil {
			a.log.WithErr(err).WithField("certificate_id", cert.ID).Error("续期证书失败")
			failCount++
		} else {
			successCount++
		}
	}

	a.log.WithFields(map[string]interface{}{
		"total":   len(certs),
		"success": successCount,
		"fail":    failCount,
	}).Info("证书续期任务完成")

	return nil
}

// updateCertError 更新证书错误信息
func (a *App) updateCertError(ctx context.Context, certificateID uint, errMsg string) {
	err := base.DB.WithContext(ctx).Model(&model.Certificate{}).
		Where("id = ?", certificateID).
		Updates(map[string]interface{}{
			"status":     model.CertificateStatusFailed,
			"last_error": errMsg,
		}).Error

	if err != nil {
		a.log.WithErr(err).WithField("certificate_id", certificateID).Error("更新证书错误信息失败")
	}
}

// GetCertificate 获取证书详情
func (a *App) GetCertificate(ctx context.Context, id uint) (*model.Certificate, error) {
	return a.CertificateDao.FindById(ctx, id)
}

// ListCertificates 查询证书列表
func (a *App) ListCertificates(ctx context.Context, page, pageSize int) ([]model.Certificate, int64, error) {
	pageInfo := &mvc.Page{
		PageNum: page,
		Size:    pageSize,
	}
	certs, total, err := a.CertificateDao.FindPage(ctx, pageInfo, nil)
	if err != nil {
		return nil, 0, err
	}
	// 转换为非指针切片
	result := make([]model.Certificate, 0, len(certs))
	for _, cert := range certs {
		result = append(result, *cert)
	}
	return result, total, nil
}

// DeleteCertificate 删除证书
func (a *App) DeleteCertificate(ctx context.Context, id uint) error {
	cert, err := a.CertificateDao.FindById(ctx, id)
	if err != nil {
		return a.err.New("获取证书失败", err)
	}

	// 删除证书及相关部署历史
	tx := base.DB.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 删除部署历史
	if err := tx.Where("certificate_id = ?", id).Delete(&model.DeployHistory{}).Error; err != nil {
		tx.Rollback()
		return a.err.New("删除部署历史失败", err)
	}

	// 删除证书
	if err := tx.Delete(cert).Error; err != nil {
		tx.Rollback()
		return a.err.New("删除证书失败", err)
	}

	return tx.Commit().Error
}
