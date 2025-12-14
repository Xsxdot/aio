package dao

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/ssl/internal/model"

	"gorm.io/gorm"
)

// DeployTargetDao 部署目标数据访问层
type DeployTargetDao struct {
	mvc.IBaseDao[model.DeployTarget]
	db  *gorm.DB
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewDeployTargetDao 创建部署目标 DAO 实例
func NewDeployTargetDao(db *gorm.DB, log *logger.Log) *DeployTargetDao {
	return &DeployTargetDao{
		IBaseDao: mvc.NewGormDao[model.DeployTarget](db),
		db:       db,
		log:      log.WithEntryName("DeployTargetDao"),
		err:      errorc.NewErrorBuilder("DeployTargetDao"),
	}
}

// FindActiveTargets 查询所有启用的部署目标
func (d *DeployTargetDao) FindActiveTargets(ctx context.Context) ([]model.DeployTarget, error) {
	var targets []model.DeployTarget
	err := d.db.WithContext(ctx).Where("status = ?", 1).Find(&targets).Error
	if err != nil {
		d.log.WithErr(err).Error("查询启用部署目标失败")
		return nil, d.err.New("查询启用部署目标失败", err)
	}
	return targets, nil
}

// FindByType 根据类型查询部署目标列表
func (d *DeployTargetDao) FindByType(ctx context.Context, targetType model.DeployTargetType) ([]model.DeployTarget, error) {
	var targets []model.DeployTarget
	err := d.db.WithContext(ctx).Where("type = ? AND status = ?", targetType, 1).Find(&targets).Error
	if err != nil {
		d.log.WithErr(err).WithField("type", targetType).Error("根据类型查询部署目标失败")
		return nil, d.err.New("根据类型查询部署目标失败", err)
	}
	return targets, nil
}

// FindActiveTargetsByCertificateDomain 根据证书域名查询候选部署目标
// 对于精确域名：查询 domain = certDomain
// 对于通配符域名（*.base）：查询 domain = *.base 或 domain LIKE '%.base'（候选集，需进一步过滤）
func (d *DeployTargetDao) FindActiveTargetsByCertificateDomain(ctx context.Context, certDomain string) ([]model.DeployTarget, error) {
	var targets []model.DeployTarget
	var err error

	if len(certDomain) > 2 && certDomain[:2] == "*." {
		// 通配符证书：*.base
		base := certDomain[2:] // 去掉 "*."
		// 查询：domain = "*.base" 或 domain LIKE "%.base"（候选集合，包含 *.base、b.base、x.b.base 等）
		err = d.db.WithContext(ctx).
			Where("status = ? AND (domain = ? OR domain LIKE ?)", 1, certDomain, "%."+base).
			Find(&targets).Error
	} else {
		// 精确域名证书：只匹配 domain = certDomain
		err = d.db.WithContext(ctx).
			Where("status = ? AND domain = ?", 1, certDomain).
			Find(&targets).Error
	}

	if err != nil {
		d.log.WithErr(err).WithField("cert_domain", certDomain).Error("根据证书域名查询部署目标失败")
		return nil, d.err.New("根据证书域名查询部署目标失败", err)
	}

	return targets, nil
}
