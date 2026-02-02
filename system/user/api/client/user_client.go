package client

import (
	"context"
	"encoding/json"
	errorc "github.com/xsxdot/aio/pkg/core/err"
	"github.com/xsxdot/aio/system/user/api/dto"
	"github.com/xsxdot/aio/system/user/internal/app"
	"github.com/xsxdot/aio/system/user/internal/model"
)

// UserClient 用户组件对外客户端（供其他组件调用）
type UserClient struct {
	app *app.App
	err *errorc.ErrorBuilder
}

// NewUserClient 创建用户客户端实例
func NewUserClient(app *app.App) *UserClient {
	return &UserClient{
		app: app,
		err: errorc.NewErrorBuilder("UserClient"),
	}
}

// GetAdminByID 根据 ID 查询管理员
func (c *UserClient) GetAdminByID(ctx context.Context, id int64) (*dto.AdminDTO, error) {
	admin, err := c.app.AdminService.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.convertAdminToDTO(admin), nil
}

// GetAdminByAccount 根据账号查询管理员
func (c *UserClient) GetAdminByAccount(ctx context.Context, account string) (*dto.AdminDTO, error) {
	admin, err := c.app.AdminService.FindByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	return c.convertAdminToDTO(admin), nil
}

// ValidateAdminLogin 验证管理员登录
func (c *UserClient) ValidateAdminLogin(ctx context.Context, account, password string) (*dto.AdminDTO, error) {
	admin, err := c.app.AdminService.ValidateLogin(ctx, account, password)
	if err != nil {
		return nil, err
	}
	return c.convertAdminToDTO(admin), nil
}

// GetClientCredentialByID 根据 ID 查询客户端凭证
func (c *UserClient) GetClientCredentialByID(ctx context.Context, id int64) (*dto.ClientCredentialDTO, error) {
	client, err := c.app.ClientCredentialService.FindById(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.convertClientToDTO(client), nil
}

// GetClientCredentialByKey 根据 key 查询客户端凭证
func (c *UserClient) GetClientCredentialByKey(ctx context.Context, clientKey string) (*dto.ClientCredentialDTO, error) {
	client, err := c.app.ClientCredentialService.FindByClientKey(ctx, clientKey)
	if err != nil {
		return nil, err
	}
	return c.convertClientToDTO(client), nil
}

// ValidateClientCredential 验证客户端凭证（用于鉴权）
func (c *UserClient) ValidateClientCredential(ctx context.Context, clientKey, clientSecret string) (*dto.ClientCredentialDTO, error) {
	client, err := c.app.ClientCredentialService.ValidateClient(ctx, clientKey, clientSecret)
	if err != nil {
		return nil, err
	}
	return c.convertClientToDTO(client), nil
}

// convertAdminToDTO 将管理员模型转换为 DTO
func (c *UserClient) convertAdminToDTO(admin *model.Admin) *dto.AdminDTO {
	return &dto.AdminDTO{
		ID:        admin.ID,
		Account:   admin.Account,
		Status:    admin.Status,
		Remark:    admin.Remark,
		CreatedAt: admin.CreatedAt,
		UpdatedAt: admin.UpdatedAt,
	}
}

// convertClientToDTO 将客户端凭证模型转换为 DTO
func (c *UserClient) convertClientToDTO(client *model.ClientCredential) *dto.ClientCredentialDTO {
	// 解析 IP 白名单
	var ipWhitelist []string
	if client.IPWhitelist != "" {
		_ = json.Unmarshal([]byte(client.IPWhitelist), &ipWhitelist)
	}

	return &dto.ClientCredentialDTO{
		ID:          client.ID,
		Name:        client.Name,
		ClientKey:   client.ClientKey,
		Status:      client.Status,
		Description: client.Description,
		IPWhitelist: ipWhitelist,
		ExpiresAt:   client.ExpiresAt,
		CreatedAt:   client.CreatedAt,
		UpdatedAt:   client.UpdatedAt,
	}
}

// convertClientToDTOWithSecret 将客户端凭证模型转换为包含 secret 的 DTO
func (c *UserClient) convertClientToDTOWithSecret(client *model.ClientCredential, secret string) *dto.ClientCredentialWithSecretDTO {
	baseDTO := c.convertClientToDTO(client)
	return &dto.ClientCredentialWithSecretDTO{
		ClientCredentialDTO: *baseDTO,
		ClientSecret:        secret,
	}
}



