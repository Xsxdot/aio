package service

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/system/ssl/internal/dao"
)

// DeployTargetService 部署目标服务
type DeployTargetService struct {
	dao       *dao.DeployTargetDao
	cryptoSvc *CryptoService
	log       *logger.Log
	err       *errorc.ErrorBuilder
}

// NewDeployTargetService 创建部署目标服务实例
func NewDeployTargetService(dao *dao.DeployTargetDao, cryptoSvc *CryptoService, log *logger.Log) *DeployTargetService {
	return &DeployTargetService{
		dao:       dao,
		cryptoSvc: cryptoSvc,
		log:       log.WithEntryName("DeployTargetService"),
		err:       errorc.NewErrorBuilder("DeployTargetService"),
	}
}

// MatchTargetsByCertificateDomain 根据证书域名匹配部署目标
// 返回符合条件的目标 ID 列表
// 规则：
//   - 证书域名为精确域名（如 b.a.com）：只匹配 domain=b.a.com 的目标
//   - 证书域名为通配符（如 *.a.com）：匹配 domain=*.a.com 以及单层子域（如 b.a.com），排除根域（a.com）和多层子域（x.b.a.com）
func (s *DeployTargetService) MatchTargetsByCertificateDomain(ctx context.Context, certDomain string) ([]uint, error) {
	// 1. 查询候选集
	candidates, err := s.dao.FindActiveTargetsByCertificateDomain(ctx, certDomain)
	if err != nil {
		return nil, err
	}

	// 2. 如果证书域名不是通配符，直接返回所有候选（已精确匹配）
	if len(certDomain) < 2 || certDomain[:2] != "*." {
		targetIDs := make([]uint, 0, len(candidates))
		for _, t := range candidates {
			targetIDs = append(targetIDs, uint(t.ID))
		}
		return targetIDs, nil
	}

	// 3. 证书域名为通配符 *.base：需要过滤候选集
	base := certDomain[2:] // 去掉 "*."
	targetIDs := make([]uint, 0, len(candidates))

	for _, t := range candidates {
		// 如果目标域名也是通配符，直接匹配
		if t.Domain == certDomain {
			targetIDs = append(targetIDs, uint(t.ID))
			continue
		}

		// 如果目标域名是精确域名，判断是否为单层子域
		if isValidSingleLevelSubdomain(t.Domain, base) {
			targetIDs = append(targetIDs, uint(t.ID))
		}
	}

	return targetIDs, nil
}

// isValidSingleLevelSubdomain 判断 targetDomain 是否是 base 的单层子域
// 例如：base="a.com"，targetDomain="b.a.com" 返回 true；targetDomain="x.b.a.com" 返回 false；targetDomain="a.com" 返回 false
func isValidSingleLevelSubdomain(targetDomain, base string) bool {
	// 1. 必须以 ".base" 结尾
	suffix := "." + base
	if !endsWithSuffix(targetDomain, suffix) {
		return false
	}

	// 2. 去掉 ".base" 后的前缀部分不能包含 "."（单层）
	prefix := targetDomain[:len(targetDomain)-len(suffix)]
	if prefix == "" {
		// 如果前缀为空，说明 targetDomain = base（根域），不匹配
		return false
	}

	// 3. 前缀中不能包含 "."（单层子域）
	for i := 0; i < len(prefix); i++ {
		if prefix[i] == '.' {
			return false
		}
	}

	return true
}

// endsWithSuffix 判断字符串是否以指定后缀结尾
func endsWithSuffix(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	return s[len(s)-len(suffix):] == suffix
}
