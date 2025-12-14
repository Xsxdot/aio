package service

import (
	"context"
	errorc "xiaozhizhang/pkg/core/err"
	"xiaozhizhang/pkg/core/logger"
	"xiaozhizhang/pkg/core/mvc"
	"xiaozhizhang/system/user/internal/dao"
	"xiaozhizhang/system/user/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminService 管理员服务
type AdminService struct {
	mvc.IBaseService[model.Admin]
	dao *dao.AdminDao
	log *logger.Log
	err *errorc.ErrorBuilder
}

// NewAdminService 创建管理员服务实例
func NewAdminService(dao *dao.AdminDao, log *logger.Log) *AdminService {
	return &AdminService{
		IBaseService: mvc.NewBaseService[model.Admin](dao.IBaseDao),
		dao:          dao,
		log:          log,
		err:          errorc.NewErrorBuilder("AdminService"),
	}
}

// FindByAccount 根据账号查询管理员
func (s *AdminService) FindByAccount(ctx context.Context, account string) (*model.Admin, error) {
	return s.dao.FindByAccount(ctx, account)
}

// CreateAdmin 创建管理员（自动散列密码）
func (s *AdminService) CreateAdmin(ctx context.Context, account, password, remark string) (*model.Admin, error) {
	// 检查账号是否已存在
	exists, err := s.dao.ExistsByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, s.err.New("管理员账号已存在", nil).ValidWithCtx()
	}

	// 散列密码
	passwordHash, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// 创建管理员
	admin := &model.Admin{
		Account:      account,
		PasswordHash: passwordHash,
		Status:       model.AdminStatusEnabled,
		Remark:       remark,
	}

	if err := s.dao.Create(ctx, admin); err != nil {
		return nil, err
	}

	return admin, nil
}

// CreateSuperAdmin 创建超级管理员（自动散列密码，设置 IsSuper=true）
func (s *AdminService) CreateSuperAdmin(ctx context.Context, account, password, remark string) (*model.Admin, error) {
	// 检查账号是否已存在
	exists, err := s.dao.ExistsByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, s.err.New("管理员账号已存在", nil).ValidWithCtx()
	}

	// 散列密码
	passwordHash, err := s.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// 创建超级管理员
	admin := &model.Admin{
		Account:      account,
		PasswordHash: passwordHash,
		Status:       model.AdminStatusEnabled,
		IsSuper:      true,
		Roles:        []string{},
		Remark:       remark,
	}

	if err := s.dao.Create(ctx, admin); err != nil {
		return nil, err
	}

	return admin, nil
}

// Count 查询管理员总数
func (s *AdminService) Count(ctx context.Context) (int64, error) {
	return s.dao.Count(ctx)
}

// UpdatePassword 更新管理员密码（需要验证旧密码）
func (s *AdminService) UpdatePassword(ctx context.Context, adminID int64, oldPassword, newPassword string) error {
	// 查询管理员
	admin, err := s.dao.FindById(ctx, adminID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if !s.VerifyPassword(oldPassword, admin.PasswordHash) {
		return s.err.New("旧密码错误", nil).ValidWithCtx()
	}

	// 散列新密码
	newPasswordHash, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// 更新密码
	return s.dao.UpdatePassword(ctx, adminID, newPasswordHash)
}

// ResetPassword 重置管理员密码（管理员操作，无需旧密码）
func (s *AdminService) ResetPassword(ctx context.Context, adminID int64, newPassword string) error {
	// 验证管理员是否存在
	_, err := s.dao.FindById(ctx, adminID)
	if err != nil {
		return err
	}

	// 散列新密码
	newPasswordHash, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}

	// 更新密码
	return s.dao.UpdatePassword(ctx, adminID, newPasswordHash)
}

// UpdateStatus 更新管理员状态
func (s *AdminService) UpdateStatus(ctx context.Context, adminID int64, status int8) error {
	// 验证状态值
	if status != model.AdminStatusEnabled && status != model.AdminStatusDisabled {
		return s.err.New("无效的状态值", nil).ValidWithCtx()
	}

	return s.dao.UpdateStatus(ctx, adminID, status)
}

// ValidateLogin 验证管理员登录
func (s *AdminService) ValidateLogin(ctx context.Context, account, password string) (*model.Admin, error) {
	// 查询管理员
	admin, err := s.dao.FindByAccount(ctx, account)
	if err != nil {
		if errorc.IsNotFound(err) {
			return nil, s.err.New("账号或密码错误", nil).ValidWithCtx()
		}
		return nil, err
	}

	// 检查状态
	if admin.Status != model.AdminStatusEnabled {
		return nil, s.err.New("管理员账号已被禁用", nil).ValidWithCtx()
	}

	// 验证密码
	if !s.VerifyPassword(password, admin.PasswordHash) {
		return nil, s.err.New("账号或密码错误", nil).ValidWithCtx()
	}

	return admin, nil
}

// FindAllActive 查询所有启用的管理员
func (s *AdminService) FindAllActive(ctx context.Context) ([]*model.Admin, error) {
	return s.dao.FindAllActive(ctx)
}

// HashPassword 散列密码
func (s *AdminService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", s.err.New("密码散列失败", err)
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func (s *AdminService) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// WithTx 使用事务
func (s *AdminService) WithTx(tx *gorm.DB) *AdminService {
	return &AdminService{
		IBaseService: s.IBaseService.WithTx(tx),
		dao:          s.dao.WithTx(tx),
		log:          s.log,
		err:          s.err,
	}
}

