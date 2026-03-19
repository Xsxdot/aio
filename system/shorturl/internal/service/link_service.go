package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/pkg/core/logger"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/shorturl/internal/dao"
	"github.com/xsxdot/aio/system/shorturl/internal/model"
)

// LinkService 短链接业务逻辑层
type LinkService struct {
	mvc.IBaseService[model.ShortLink]
	Dao *dao.LinkDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewLinkService 创建短链接服务实例
func NewLinkService(daoInstance *dao.LinkDao, log *logger.Log) *LinkService {
	return &LinkService{
		IBaseService: mvc.NewBaseService[model.ShortLink](daoInstance),
		Dao:          daoInstance,
		log:          log.WithEntryName("LinkService"),
		err:          errorc.NewErrorBuilder("LinkService"),
	}
}

// GenerateUniqueCode 生成唯一短码（带冲突重试）
func (s *LinkService) GenerateUniqueCode(ctx context.Context, domainID int64, codeLength int, maxRetries int) (string, error) {
	if codeLength <= 0 {
		codeLength = 6
	}
	if maxRetries <= 0 {
		maxRetries = 10
	}

	for i := 0; i < maxRetries; i++ {
		code, err := GenerateShortCode(codeLength)
		if err != nil {
			return "", s.err.New("生成短码失败", err)
		}

		exists, err := s.Dao.ExistsByDomainAndCode(ctx, domainID, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}

	return "", s.err.New("生成唯一短码失败（超过重试次数）", nil)
}

// HashPassword 对密码进行哈希
func (s *LinkService) HashPassword(password string) string {
	if password == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// VerifyPassword 验证密码
func (s *LinkService) VerifyPassword(password, passwordHash string) bool {
	if passwordHash == "" {
		return true // 未设置密码
	}
	return s.HashPassword(password) == passwordHash
}

// ValidateLink 验证短链接是否可访问
func (s *LinkService) ValidateLink(link *model.ShortLink, password string) error {
	if !link.Enabled {
		return s.err.New("短链接已禁用", nil).Forbidden()
	}

	if link.IsExpired() {
		return s.err.New("短链接已过期", nil).Forbidden()
	}

	if link.IsVisitLimitReached() {
		return s.err.New("短链接访问次数已达上限", nil).Forbidden()
	}

	if link.HasPassword() && !s.VerifyPassword(password, link.PasswordHash) {
		return s.err.New("访问密码错误", nil).Forbidden()
	}

	return nil
}

// IncrementVisitCount 增加访问次数
func (s *LinkService) IncrementVisitCount(ctx context.Context, linkID int64) error {
	return s.Dao.IncrementVisitCount(ctx, linkID)
}

// IncrementSuccessCount 增加成功次数
func (s *LinkService) IncrementSuccessCount(ctx context.Context, linkID int64) error {
	return s.Dao.IncrementSuccessCount(ctx, linkID)
}

// ExistsByDomainAndCode 检查域下短码是否存在
func (s *LinkService) ExistsByDomainAndCode(ctx context.Context, domainID int64, code string) (bool, error) {
	return s.Dao.ExistsByDomainAndCode(ctx, domainID, code)
}

// FindByDomainAndCode 根据域名和短码查找短链接
func (s *LinkService) FindByDomainAndCode(ctx context.Context, domainID int64, code string) (*model.ShortLink, error) {
	return s.Dao.FindByDomainAndCode(ctx, domainID, code)
}

// Save 保存短链接实体
func (s *LinkService) Save(ctx context.Context, link *model.ShortLink) error {
	if err := s.Dao.DB.WithContext(ctx).Save(link).Error; err != nil {
		return s.err.New("保存短链接失败", err).DB()
	}
	return nil
}

// FindByCode 根据短码查找短链接
func (s *LinkService) FindByCode(ctx context.Context, code string) (*model.ShortLink, error) {
	return s.Dao.FindByCode(ctx, code)
}







