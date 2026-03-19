package mvc

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// WithTxToContext 将事务连接存入 context，用于上下级链路事务隐式传递
func WithTxToContext(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// ExtractDB 优先从 context 中提取事务连接，如果不存在事务，会自动降级 fallback 到传入的 defaultDB 并附加 ctx
func ExtractDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return defaultDB.WithContext(ctx)
}
