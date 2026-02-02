package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
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

// RecordVisit 记录访问（需配合 DAO 的原子递增）
func (s *LinkService) RecordVisit(ctx context.Context, linkID int64, visitDao *dao.VisitDao, ip, ua, referer string) error {
	// 创建访问记录
	visit := &model.ShortVisit{
		LinkID:    linkID,
		IP:        ip,
		UserAgent: ua,
		Referer:   referer,
		VisitedAt: time.Now(),
	}

	if err := visitDao.Create(ctx, visit); err != nil {
		s.log.WithErr(err).Error("创建访问记录失败")
		// 不阻断流程，记录失败仅打日志
	}

	// 原子递增访问次数
	return s.Dao.IncrementVisitCount(ctx, linkID)
}

// RecordSuccess 记录成功上报
func (s *LinkService) RecordSuccess(ctx context.Context, linkID int64, successDao *dao.SuccessEventDao, eventID string, attrs map[string]interface{}) error {
	// 检查eventID是否已存在（幂等）
	if eventID != "" {
		exists, err := successDao.ExistsByEventID(ctx, eventID)
		if err != nil {
			return err
		}
		if exists {
			s.log.WithField("event_id", eventID).Info("事件ID已存在，跳过重复上报")
			return nil // 幂等，不报错
		}
	}

	// 创建成功事件
	event := &model.ShortSuccessEvent{
		LinkID:  linkID,
		EventID: eventID,
		Attrs:   attrs,
	}

	if err := successDao.Create(ctx, event); err != nil {
		return err
	}

	// 原子递增成功次数
	return s.Dao.IncrementSuccessCount(ctx, linkID)
}

