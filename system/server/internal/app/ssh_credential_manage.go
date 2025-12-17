package app

import (
	"context"
	"xiaozhizhang/system/server/internal/model"
)

// UpsertServerSSHCredential 更新或插入 SSH 凭证
func (a *App) UpsertServerSSHCredential(ctx context.Context, credential *model.ServerSSHCredential) error {
	return a.ServerSSHCredentialSvc.Upsert(ctx, credential)
}

// GetDecryptedServerSSHCredential 获取解密后的 SSH 凭证
func (a *App) GetDecryptedServerSSHCredential(ctx context.Context, serverID int64) (*model.ServerSSHCredential, error) {
	return a.ServerSSHCredentialSvc.GetDecrypted(ctx, serverID)
}

// DeleteServerSSHCredential 删除 SSH 凭证
func (a *App) DeleteServerSSHCredential(ctx context.Context, serverID int64) error {
	return a.ServerSSHCredentialSvc.Delete(ctx, serverID)
}
