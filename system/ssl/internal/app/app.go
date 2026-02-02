package app

import (
	"github.com/xsxdot/aio/base"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/system/ssl/internal/dao"
	"github.com/xsxdot/aio/system/ssl/internal/facade"
	"github.com/xsxdot/aio/system/ssl/internal/service"
)

// App SSL 证书组件应用层
// 负责组合/调度 Service，实现复杂业务逻辑
type App struct {
	// DAOs
	DnsCredentialDao *dao.DnsCredentialDao
	CertificateDao   *dao.CertificateDao
	DeployTargetDao  *dao.DeployTargetDao
	DeployHistoryDao *dao.DeployHistoryDao

	// Services
	AcmeService     *service.AcmeService
	CryptoService   *service.CryptoService
	DeployService   *service.DeployService
	DnsCredSvc      *service.DnsCredentialService
	CertificateSvc  *service.CertificateService
	DeployTargetSvc *service.DeployTargetService

	// Facades (跨组件依赖)
	serverFacade facade.IServerFacade

	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewApp 创建 SSL 证书应用层实例
func NewApp(serverFacade facade.IServerFacade) *App {
	log := logger.GetLogger().WithEntryName("SslApp")

	// 初始化 DAOs
	dnsCredDao := dao.NewDnsCredentialDao(base.DB, log)
	certDao := dao.NewCertificateDao(base.DB, log)
	deployTargetDao := dao.NewDeployTargetDao(base.DB, log)
	deployHistoryDao := dao.NewDeployHistoryDao(base.DB, log)

	// 初始化 Services
	cryptoSvc := service.NewCryptoService(log)
	acmeSvc := service.NewAcmeService(log)
	deploySvc := service.NewDeployService(log, cryptoSvc)
	dnsCredSvc := service.NewDnsCredentialService(dnsCredDao, cryptoSvc, log)
	certSvc := service.NewCertificateService(certDao, log)
	deployTargetSvc := service.NewDeployTargetService(deployTargetDao, cryptoSvc, log)

	return &App{
		DnsCredentialDao: dnsCredDao,
		CertificateDao:   certDao,
		DeployTargetDao:  deployTargetDao,
		DeployHistoryDao: deployHistoryDao,
		AcmeService:      acmeSvc,
		CryptoService:    cryptoSvc,
		DeployService:    deploySvc,
		DnsCredSvc:       dnsCredSvc,
		CertificateSvc:   certSvc,
		DeployTargetSvc:  deployTargetSvc,
		serverFacade:     serverFacade,
		log:              log,
		err:              errorc.NewErrorBuilder("SslApp"),
	}
}
