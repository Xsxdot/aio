package mvc

import (
	"context"
	errorc "github.com/xsxdot/aio/pkg/core/err"

	"gorm.io/gorm"
)

// GormDaoImpl GORM数据访问实现
type GormDaoImpl[T any] struct {
	db *gorm.DB
}

// NewGormDao 创建GORM数据访问实例
func NewGormDao[T any](db *gorm.DB) IBaseDao[T] {
	return &GormDaoImpl[T]{
		db: db,
	}
}

// WithTx 使用事务创建临时的IBaseDao实例
func (d *GormDaoImpl[T]) WithTx(tx interface{}) IBaseDao[T] {
	if gormDB, ok := tx.(*gorm.DB); ok {
		return &GormDaoImpl[T]{
			db: gormDB,
		}
	}
	return d
}

func (d *GormDaoImpl[T]) Create(ctx context.Context, entity *T) error {
	err := d.db.WithContext(ctx).Create(entity).Error
	if err != nil {
		return errorc.New("数据库操作失败", err).DB()
	}
	return nil
}

func (d *GormDaoImpl[T]) CreateBatch(ctx context.Context, entities []*T) error {
	err := d.db.WithContext(ctx).Create(entities).Error
	if err != nil {
		return errorc.New("批量创建记录失败", err).DB()
	}
	return nil
}

func (d *GormDaoImpl[T]) DeleteById(ctx context.Context, id interface{}) error {
	result := d.db.WithContext(ctx).Delete(new(T), id)
	if result.Error != nil {
		return errorc.New("删除记录失败", result.Error).DB()
	}
	if result.RowsAffected == 0 {
		return errorc.New("要删除的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return nil
}

func (d *GormDaoImpl[T]) DeleteByIds(ctx context.Context, ids []interface{}) (int64, error) {
	result := d.db.WithContext(ctx).Delete(new(T), ids)
	if result.Error != nil {
		return 0, errorc.New("批量删除记录失败", result.Error).DB()
	}
	if result.RowsAffected == 0 {
		return 0, errorc.New("要删除的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return result.RowsAffected, nil
}

func (d *GormDaoImpl[T]) DeleteByColumn(ctx context.Context, column string, value interface{}) error {
	result := d.db.WithContext(ctx).Where(column+" = ?", value).Delete(new(T))
	if result.Error != nil {
		return errorc.New("批量删除记录失败", result.Error).DB()
	}
	if result.RowsAffected == 0 {
		return errorc.New("要删除的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return nil
}

func (d *GormDaoImpl[T]) DeleteByMap(ctx context.Context, conditions map[string]interface{}) error {
	result := d.db.WithContext(ctx).Where(conditions).Delete(new(T))
	if result.Error != nil {
		return errorc.New("批量删除记录失败", result.Error).DB()
	}
	if result.RowsAffected == 0 {
		return errorc.New("要删除的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return nil
}

func (d *GormDaoImpl[T]) UpdateById(ctx context.Context, id interface{}, entity *T) (int64, error) {
	result := d.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(entity)
	err := result.Error
	if err != nil {
		return 0, errorc.New("更新记录失败", err).DB()
	}
	affected := result.RowsAffected
	if affected == 0 {
		return 0, errorc.New("要更新的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return affected, err
}

func (d *GormDaoImpl[T]) UpdateByIds(ctx context.Context, ids []interface{}, entity *T) (int64, error) {
	result := d.db.WithContext(ctx).Model(new(T)).Where("id IN ?", ids).Updates(entity)
	err := result.Error
	if err != nil {
		return 0, errorc.New("更新记录失败", err).DB()
	}
	affected := result.RowsAffected
	if affected == 0 {
		return 0, errorc.New("要更新的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return affected, err
}

func (d *GormDaoImpl[T]) UpdateByColumn(ctx context.Context, column string, value interface{}, entity *T) (int64, error) {
	result := d.db.WithContext(ctx).Model(new(T)).Where(column+" = ?", value).Updates(entity)
	err := result.Error
	if err != nil {
		return 0, errorc.New("更新记录失败", err).DB()
	}
	affected := result.RowsAffected
	if affected == 0 {
		return 0, errorc.New("要更新的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return affected, err
}

func (d *GormDaoImpl[T]) UpdateByMap(ctx context.Context, conditions map[string]interface{}, entity *T) (int64, error) {
	result := d.db.WithContext(ctx).Model(new(T)).Where(conditions).Updates(entity)
	err := result.Error
	if err != nil {
		return 0, errorc.New("更新记录失败", err).DB()
	}
	affected := result.RowsAffected
	if affected == 0 {
		return 0, errorc.New("要更新的记录不存在", nil).WithCode(errorc.ErrorCodeNotFound)
	}
	return affected, err
}

func (d *GormDaoImpl[T]) FindById(ctx context.Context, id interface{}) (*T, error) {
	var entity T
	err := d.db.WithContext(ctx).First(&entity, id).Error
	if err != nil {
		return nil, errorc.New("查询记录失败", err).DB()
	}
	return &entity, nil
}

func (d *GormDaoImpl[T]) FindByIds(ctx context.Context, ids []interface{}) ([]*T, error) {
	var entities []*T
	err := d.db.WithContext(ctx).Find(&entities, ids).Error
	if err != nil {
		return nil, errorc.New("批量查询记录失败", err).DB()
	}
	return entities, nil
}

func (d *GormDaoImpl[T]) FindByColumn(ctx context.Context, column string, value interface{}) ([]*T, error) {
	var entities []*T
	err := d.db.WithContext(ctx).Where(column+" = ?", value).Find(&entities).Error
	if err != nil {
		return nil, errorc.New("查询记录失败", err).DB()
	}
	return entities, nil
}

func (d *GormDaoImpl[T]) FindByUserId(ctx context.Context, userId interface{}) ([]*T, error) {
	var entities []*T
	err := d.db.WithContext(ctx).Where("user_id = ?", userId).Find(&entities).Error
	if err != nil {
		return nil, errorc.New("查询记录失败", err).DB()
	}
	return entities, nil
}

func (d *GormDaoImpl[T]) FindOneByColumn(ctx context.Context, column string, value interface{}) (*T, error) {
	var entity T
	err := d.db.WithContext(ctx).Where(column+" = ?", value).First(&entity).Error
	if err != nil {
		return nil, errorc.New("查询记录失败", err).DB()
	}
	return &entity, nil
}

func (d *GormDaoImpl[T]) FindByMap(ctx context.Context, conditions map[string]interface{}) ([]*T, error) {
	var entities []*T
	err := d.db.WithContext(ctx).Where(conditions).Find(&entities).Error
	if err != nil {
		return nil, errorc.New("查询记录失败", err).DB()
	}
	return entities, nil
}

func (d *GormDaoImpl[T]) FindOneByMap(ctx context.Context, conditions map[string]interface{}) (*T, error) {
	var entity T
	err := d.db.WithContext(ctx).Where(conditions).First(&entity).Error
	if err != nil {
		return nil, errorc.New("查询记录失败", err).DB()
	}
	return &entity, nil
}

func (d *GormDaoImpl[T]) FindList(ctx context.Context, condition *T) ([]*T, error) {
	var entities []*T
	err := d.db.WithContext(ctx).Where(condition).Find(&entities).Error
	if err != nil {
		return nil, errorc.New("查询记录失败", err).DB()
	}
	return entities, nil
}

func (d *GormDaoImpl[T]) FindPage(ctx context.Context, page *Page, condition *T) ([]*T, int64, error) {
	var entities []*T
	var total int64

	db := d.db.WithContext(ctx).Model(new(T))
	if condition != nil {
		db = db.Where(condition)
	}

	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, errorc.New("查询记录失败", err).DB()
	}

	pageNum := page.PageNum
	size := page.Size

	if pageNum == 0 {
		pageNum = 1
	}

	if size <= 0 {
		size = 10
	}

	// 设置分页和排序条件
	db = db.Offset((pageNum - 1) * size).Limit(size)

	// 如果Page结构体中包含排序字段，则设置排序条件
	if page.Sort != nil {
		db = db.Order(page.Sort)
	}

	err = db.Find(&entities).Error
	if err != nil {
		return nil, 0, errorc.New("查询记录失败", err).DB()
	}

	return entities, total, nil
}

func (d *GormDaoImpl[T]) FindPageByMap(ctx context.Context, page *Page, condition map[string]interface{}) ([]*T, int64, error) {
	var entities []*T
	var total int64

	db := d.db.WithContext(ctx).Model(new(T))
	if condition != nil {
		db = db.Where(condition)
	}

	err := db.Count(&total).Error
	if err != nil {
		return nil, 0, errorc.New("查询记录失败", err).DB()
	}

	pageNum := page.PageNum
	size := page.Size

	if pageNum == 0 {
		pageNum = 1
	}

	if size <= 0 {
		size = 10
	}

	// 设置分页和排序条件
	db = db.Offset((pageNum - 1) * size).Limit(size)

	// 如果Page结构体中包含排序字段，则设置排序条件
	if page.Sort != nil {
		db = db.Order(page.Sort)
	}

	err = db.Find(&entities).Error
	if err != nil {
		return nil, 0, errorc.New("查询记录失败", err).DB()
	}

	return entities, total, nil
}

func (d *GormDaoImpl[T]) Count(ctx context.Context, condition *T) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(new(T)).Where(condition).Count(&count).Error
	if err != nil {
		return 0, errorc.New("查询记录失败", err).DB()
	}
	return count, err
}

func (d *GormDaoImpl[T]) CountByMap(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(new(T)).Where(conditions).Count(&count).Error
	if err != nil {
		return 0, errorc.New("查询记录失败", err).DB()
	}
	return count, err
}

func (d *GormDaoImpl[T]) Exists(ctx context.Context, condition *T) (bool, error) {
	count, err := d.Count(ctx, condition)
	return count > 0, err
}

func (d *GormDaoImpl[T]) ExistsByMap(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	count, err := d.CountByMap(ctx, conditions)
	return count > 0, err
}
