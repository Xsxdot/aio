package service

import (
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/ssl/internal/dao"
)

// CertificateService 证书服务
type CertificateService struct {
	dao *dao.CertificateDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewCertificateService 创建证书服务实例
func NewCertificateService(dao *dao.CertificateDao, log *logger.Log) *CertificateService {
	return &CertificateService{
		dao: dao,
		log: log.WithEntryName("CertificateService"),
		err: errorc.NewErrorBuilder("CertificateService"),
	}
}
