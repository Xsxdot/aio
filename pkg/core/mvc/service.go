package mvc

import (
	"context"
)

// IBaseService 基础服务接口
type IBaseService[T any] interface {
	// Create 创建实体
	Create(ctx context.Context, entity *T) error
	// CreateBatch 批量创建实体
	CreateBatch(ctx context.Context, entities []*T) error
	// DeleteById 根据ID删除
	DeleteById(ctx context.Context, id interface{}) error
	// DeleteByIds 根据ID批量删除
	DeleteByIds(ctx context.Context, ids []interface{}) (int64, error)
	// UpdateById 根据ID更新
	UpdateById(ctx context.Context, id interface{}, entity *T) (int64, error)
	// FindById 根据ID查询
	FindById(ctx context.Context, id interface{}) (*T, error)
	// FindPage 分页查询
	FindPage(ctx context.Context, page *Page, condition *T) ([]*T, int64, error)
	// FindPageWithMap 分页查询
	FindPageWithMap(ctx context.Context, page *Page, condition map[string]interface{}) ([]*T, int64, error)
	// FindByUserId 根据用户ID查询
	FindByUserId(ctx context.Context, userId interface{}) ([]*T, error)
	// WithTx 设置事务
	WithTx(tx interface{}) IBaseService[T]
}

// BaseService 基础服务实现
type BaseService[T any] struct {
	Dao IBaseDao[T]
}

// NewBaseService 创建基础服务实例
func NewBaseService[T any](dao IBaseDao[T]) *BaseService[T] {
	return &BaseService[T]{
		Dao: dao,
	}
}

func (s *BaseService[T]) WithTx(tx interface{}) IBaseService[T] {
	return NewBaseService[T](s.Dao.WithTx(tx))
}

func (s *BaseService[T]) Create(ctx context.Context, entity *T) error {
	return s.Dao.Create(ctx, entity)
}

func (s *BaseService[T]) CreateBatch(ctx context.Context, entities []*T) error {
	return s.Dao.CreateBatch(ctx, entities)
}

func (s *BaseService[T]) DeleteById(ctx context.Context, id interface{}) error {
	return s.Dao.DeleteById(ctx, id)
}

func (s *BaseService[T]) DeleteByIds(ctx context.Context, ids []interface{}) (int64, error) {
	return s.Dao.DeleteByIds(ctx, ids)
}

func (s *BaseService[T]) UpdateById(ctx context.Context, id interface{}, entity *T) (int64, error) {
	return s.Dao.UpdateById(ctx, id, entity)
}

func (s *BaseService[T]) FindById(ctx context.Context, id interface{}) (*T, error) {
	return s.Dao.FindById(ctx, id)
}

func (s *BaseService[T]) FindPage(ctx context.Context, page *Page, condition *T) ([]*T, int64, error) {
	return s.Dao.FindPage(ctx, page, condition)
}

func (s *BaseService[T]) FindPageWithMap(ctx context.Context, page *Page, condition map[string]interface{}) ([]*T, int64, error) {
	return s.Dao.FindPageByMap(ctx, page, condition)
}

func (s *BaseService[T]) FindByUserId(ctx context.Context, userId interface{}) ([]*T, error) {
	return s.Dao.FindByUserId(ctx, userId)
}
