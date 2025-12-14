package app

import (
	"xiaozhizhang/base"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/application/internal/dao"
	"xiaozhizhang/system/application/internal/model"
	"xiaozhizhang/system/application/internal/service"
	"xiaozhizhang/system/application/internal/service/storage"
	"xiaozhizhang/system/nginx"
	"xiaozhizhang/system/registry"
	"xiaozhizhang/system/ssl"
	"xiaozhizhang/system/systemd"

	"gorm.io/gorm"
)

// App Application 组件应用层
// 负责组合/调度 Service，实现复杂业务逻辑
type App struct {
	// DAOs
	ApplicationDao *dao.ApplicationDao
	ArtifactDao    *dao.ArtifactDao
	ReleaseDao     *dao.ReleaseDao
	DeploymentDao  *dao.DeploymentDao

	// Services
	ApplicationSvc *service.ApplicationService
	ArtifactSvc    *service.ArtifactService
	ReleaseSvc     *service.ReleaseService
	DeploymentSvc  *service.DeploymentService

	// Storage
	Storage     storage.Storage
	StorageMode model.StorageMode

	// 配置
	ReleaseDir   string
	KeepReleases int

	// 依赖的其他组件
	SslModule      *ssl.Module
	NginxModule    *nginx.Module
	SystemdModule  *systemd.Module
	RegistryModule *registry.Module

	log *logger.Log
	err *errorc.ErrorBuilder
	db  *gorm.DB
}

// NewApp 创建 Application 组件应用层实例
func NewApp(
	sslModule *ssl.Module,
	nginxModule *nginx.Module,
	systemdModule *systemd.Module,
	registryModule *registry.Module,
) *App {
	log := base.Logger.WithEntryName("ApplicationApp")

	// 初始化 DAOs
	applicationDao := dao.NewApplicationDao(base.DB, log)
	artifactDao := dao.NewArtifactDao(base.DB, log)
	releaseDao := dao.NewReleaseDao(base.DB, log)
	deploymentDao := dao.NewDeploymentDao(base.DB, log)

	// 初始化 Services
	applicationSvc := service.NewApplicationService(applicationDao, log)
	artifactSvc := service.NewArtifactService(artifactDao, log)
	releaseSvc := service.NewReleaseService(releaseDao, log)
	deploymentSvc := service.NewDeploymentService(deploymentDao, log)

	// 初始化 Storage（根据配置选择 local 或 oss）
	appConfig := base.Configures.Config.Application
	storageMode := model.StorageModeLocal
	releaseDir := "/opt/apps/releases"
	keepReleases := 5

	if appConfig.StorageMode != "" {
		storageMode = model.StorageMode(appConfig.StorageMode)
	}
	if appConfig.ReleaseDir != "" {
		releaseDir = appConfig.ReleaseDir
	}
	if appConfig.KeepReleases > 0 {
		keepReleases = appConfig.KeepReleases
	}

	var storageImpl storage.Storage
	if storageMode == model.StorageModeOSS && base.OSS != nil {
		storageImpl = storage.NewOSSStorage(base.OSS, appConfig.OSSPrefix, log)
	} else {
		artifactDir := appConfig.LocalArtifactDir
		if artifactDir == "" {
			artifactDir = "/opt/apps/artifacts"
		}
		var err error
		storageImpl, err = storage.NewLocalStorage(artifactDir, log)
		if err != nil {
			log.WithErr(err).Warn("初始化本地存储失败，使用默认路径")
			storageImpl, _ = storage.NewLocalStorage("/tmp/apps/artifacts", log)
		}
		storageMode = model.StorageModeLocal
	}

	return &App{
		ApplicationDao: applicationDao,
		ArtifactDao:    artifactDao,
		ReleaseDao:     releaseDao,
		DeploymentDao:  deploymentDao,
		ApplicationSvc: applicationSvc,
		ArtifactSvc:    artifactSvc,
		ReleaseSvc:     releaseSvc,
		DeploymentSvc:  deploymentSvc,
		Storage:        storageImpl,
		StorageMode:    storageMode,
		ReleaseDir:     releaseDir,
		KeepReleases:   keepReleases,
		SslModule:      sslModule,
		NginxModule:    nginxModule,
		SystemdModule:  systemdModule,
		RegistryModule: registryModule,
		log:            log,
		err:            errorc.NewErrorBuilder("ApplicationApp"),
		db:             base.DB,
	}
}

