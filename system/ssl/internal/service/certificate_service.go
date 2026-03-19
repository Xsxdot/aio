package service

import (
	"context"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/ssl/internal/dao"
	"github.com/xsxdot/aio/system/ssl/internal/model"
)

// CertificateService 证书服务
type CertificateService struct {
	mvc.IBaseService[model.Certificate]
	Dao *dao.CertificateDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewCertificateService 创建证书服务实例
func NewCertificateService(daoInstance *dao.CertificateDao, log *logger.Log) *CertificateService {
	return &CertificateService{
		IBaseService: mvc.NewBaseService[model.Certificate](daoInstance),
		Dao:          daoInstance,
		log:          log.WithEntryName("CertificateService"),
		err:          errorc.NewErrorBuilder("CertificateService"),
	}
}

// FindCertificatesToRenew 查询需要续期的证书
func (s *CertificateService) FindCertificatesToRenew(ctx context.Context) ([]model.Certificate, error) {
	return s.Dao.FindCertificatesToRenew(ctx)
}

// UpdateStatus 更新证书状态
func (s *CertificateService) UpdateStatus(ctx context.Context, id uint, status model.CertificateStatus) error {
	return s.Dao.UpdateStatus(ctx, id, status)
}

// UpdateCertError 更新证书错误信息
func (s *CertificateService) UpdateCertError(ctx context.Context, certificateID uint, errMsg string) {
	s.Dao.UpdateCertError(ctx, certificateID, errMsg)
}

// DeleteCertificateWithHistory 删除证书及其部署历史
func (s *CertificateService) DeleteCertificateWithHistory(ctx context.Context, cert *model.Certificate) error {
	return s.Dao.DeleteCertificateWithHistory(ctx, cert)
}
