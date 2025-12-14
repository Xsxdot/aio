package mvc

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// IBaseDao 定义通用的数据访问接口
type IBaseDao[T any] interface {
	// Create 创建记录
	Create(ctx context.Context, entity *T) error
	// CreateBatch 批量创建记录
	CreateBatch(ctx context.Context, entities []*T) error
	// DeleteById 根据ID删除记录
	DeleteById(ctx context.Context, id interface{}) error
	// DeleteByIds 根据ID批量删除记录
	DeleteByIds(ctx context.Context, ids []interface{}) (int64, error)
	// DeleteByColumn 根据指定列删除记录
	DeleteByColumn(ctx context.Context, column string, value interface{}) error
	// DeleteByMap 根据多个条件删除记录
	DeleteByMap(ctx context.Context, conditions map[string]interface{}) error
	// UpdateById 根据ID更新记录
	UpdateById(ctx context.Context, id interface{}, entity *T) (int64, error)
	// UpdateByIds 根据ID批量更新记录
	UpdateByIds(ctx context.Context, ids []interface{}, entity *T) (int64, error)
	// UpdateByColumn 根据指定列更新记录
	UpdateByColumn(ctx context.Context, column string, value interface{}, entity *T) (int64, error)
	// UpdateByMap 根据多个条件更新记录
	UpdateByMap(ctx context.Context, conditions map[string]interface{}, entity *T) (int64, error)
	// FindById 根据ID查询记录
	FindById(ctx context.Context, id interface{}) (*T, error)
	// FindByIds 根据ID批量查询记录
	FindByIds(ctx context.Context, ids []interface{}) ([]*T, error)
	// FindByColumn 根据指定列查询记录
	FindByColumn(ctx context.Context, column string, value interface{}) ([]*T, error)
	// FindOneByColumn 根据指定列查询单条记录
	FindOneByColumn(ctx context.Context, column string, value interface{}) (*T, error)
	// FindByMap 根据多个条件查询记录
	FindByMap(ctx context.Context, conditions map[string]interface{}) ([]*T, error)
	// FindOneByMap 根据多个条件查询单条记录
	FindOneByMap(ctx context.Context, conditions map[string]interface{}) (*T, error)
	// FindList 查询列表
	FindList(ctx context.Context, condition *T) ([]*T, error)
	// FindPage 分页查询
	FindPage(ctx context.Context, page *Page, condition *T) ([]*T, int64, error)
	// FindPageByMap 分页查询
	FindPageByMap(ctx context.Context, page *Page, condition map[string]interface{}) ([]*T, int64, error)
	// Count 统计记录数
	Count(ctx context.Context, condition *T) (int64, error)
	// CountByMap 根据多个条件统计记录数
	CountByMap(ctx context.Context, conditions map[string]interface{}) (int64, error)
	// Exists 判断记录是否存在
	Exists(ctx context.Context, condition *T) (bool, error)
	// ExistsByMap 根据多个条件判断记录是否存在
	ExistsByMap(ctx context.Context, conditions map[string]interface{}) (bool, error)
	// FindByUserId 根据用户ID查询记录
	FindByUserId(ctx context.Context, userId interface{}) ([]*T, error)
	// WithTx 使用事务创建临时的IBaseDao实例
	WithTx(tx interface{}) IBaseDao[T]
}

// MongoDaoImpl MongoDB数据访问实现
type MongoDaoImpl[T any] struct {
	coll *mongo.Collection
}

// NewMongoDao 创建MongoDB数据访问实例
func NewMongoDao[T any](coll *mongo.Collection) IBaseDao[T] {
	return &MongoDaoImpl[T]{
		coll: coll,
	}
}

// WithTx 使用事务创建临时的IBaseDao实例
func (d *MongoDaoImpl[T]) WithTx(tx interface{}) IBaseDao[T] {
	return &MongoDaoImpl[T]{
		coll: d.coll,
	}
}

func (d *MongoDaoImpl[T]) Create(ctx context.Context, entity *T) error {
	_, err := d.coll.InsertOne(ctx, entity)
	return err
}

func (d *MongoDaoImpl[T]) CreateBatch(ctx context.Context, entities []*T) error {
	docs := make([]interface{}, len(entities))
	for i, entity := range entities {
		docs[i] = entity
	}
	_, err := d.coll.InsertMany(ctx, docs)
	return err
}

func (d *MongoDaoImpl[T]) DeleteById(ctx context.Context, id interface{}) error {
	filter := bson.M{"_id": toObjectId(id)}
	_, err := d.coll.DeleteOne(ctx, filter)
	return err
}

func (d *MongoDaoImpl[T]) DeleteByIds(ctx context.Context, ids []interface{}) (int64, error) {
	objectIds := make([]primitive.ObjectID, len(ids))
	for i, id := range ids {
		objectIds[i] = toObjectId(id)
	}
	filter := bson.M{"_id": bson.M{"$in": objectIds}}
	_, err := d.coll.DeleteMany(ctx, filter)
	return 0, err
}

func (d *MongoDaoImpl[T]) DeleteByColumn(ctx context.Context, column string, value interface{}) error {
	filter := bson.M{column: value}
	_, err := d.coll.DeleteMany(ctx, filter)
	return err
}

func (d *MongoDaoImpl[T]) DeleteByMap(ctx context.Context, conditions map[string]interface{}) error {
	_, err := d.coll.DeleteMany(ctx, conditions)
	return err
}

func (d *MongoDaoImpl[T]) UpdateById(ctx context.Context, id interface{}, entity *T) (int64, error) {
	filter := bson.M{"_id": toObjectId(id)}
	result, err := d.coll.UpdateOne(ctx, filter, bson.M{"$set": entity})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (d *MongoDaoImpl[T]) UpdateByIds(ctx context.Context, ids []interface{}, entity *T) (int64, error) {
	objectIds := make([]primitive.ObjectID, len(ids))
	for i, id := range ids {
		objectIds[i] = toObjectId(id)
	}
	filter := bson.M{"_id": bson.M{"$in": objectIds}}
	result, err := d.coll.UpdateMany(ctx, filter, bson.M{"$set": entity})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (d *MongoDaoImpl[T]) UpdateByColumn(ctx context.Context, column string, value interface{}, entity *T) (int64, error) {
	filter := bson.M{column: value}
	result, err := d.coll.UpdateMany(ctx, filter, bson.M{"$set": entity})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (d *MongoDaoImpl[T]) UpdateByMap(ctx context.Context, conditions map[string]interface{}, entity *T) (int64, error) {
	result, err := d.coll.UpdateMany(ctx, conditions, bson.M{"$set": entity})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (d *MongoDaoImpl[T]) FindById(ctx context.Context, id interface{}) (*T, error) {
	filter := bson.M{"_id": toObjectId(id)}
	var entity T
	err := d.coll.FindOne(ctx, filter).Decode(&entity)
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (d *MongoDaoImpl[T]) FindByIds(ctx context.Context, ids []interface{}) ([]*T, error) {
	objectIds := make([]primitive.ObjectID, len(ids))
	for i, id := range ids {
		objectIds[i] = toObjectId(id)
	}
	filter := bson.M{"_id": bson.M{"$in": objectIds}}
	return d.find(ctx, filter)
}

func (d *MongoDaoImpl[T]) FindByColumn(ctx context.Context, column string, value interface{}) ([]*T, error) {
	filter := bson.M{column: value}
	return d.find(ctx, filter)
}

func (d *MongoDaoImpl[T]) FindByUserId(ctx context.Context, userId interface{}) ([]*T, error) {
	filter := bson.M{"user_id": userId}
	return d.find(ctx, filter)
}

func (d *MongoDaoImpl[T]) FindOneByColumn(ctx context.Context, column string, value interface{}) (*T, error) {
	filter := bson.M{column: value}
	var entity T
	err := d.coll.FindOne(ctx, filter).Decode(&entity)
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (d *MongoDaoImpl[T]) FindByMap(ctx context.Context, conditions map[string]interface{}) ([]*T, error) {
	return d.find(ctx, conditions)
}

func (d *MongoDaoImpl[T]) FindOneByMap(ctx context.Context, conditions map[string]interface{}) (*T, error) {
	var entity T
	err := d.coll.FindOne(ctx, conditions).Decode(&entity)
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

func (d *MongoDaoImpl[T]) FindList(ctx context.Context, condition *T) ([]*T, error) {
	return d.find(ctx, condition)
}

func (d *MongoDaoImpl[T]) FindPage(ctx context.Context, page *Page, condition *T) ([]*T, int64, error) {
	filter := condition
	total, err := d.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	pageNum := page.PageNum
	size := page.Size

	if pageNum == 0 {
		pageNum = 1
	}

	if size <= 0 {
		size = 10
	}

	opts := options.Find().SetSkip(int64((pageNum - 1) * size)).SetLimit(int64(size))

	// 如果Page结构体中包含排序字段，则设置排序条件
	if page.Sort != nil {
		opts.SetSort(page.Sort)
	}

	cursor, err := d.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var entities []*T
	if err = cursor.All(ctx, &entities); err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}

func (d *MongoDaoImpl[T]) FindPageByMap(ctx context.Context, page *Page, condition map[string]interface{}) ([]*T, int64, error) {
	filter := condition
	total, err := d.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	pageNum := page.PageNum
	size := page.Size

	if pageNum == 0 {
		pageNum = 1
	}

	if size <= 0 {
		size = 10
	}

	opts := options.Find().SetSkip(int64((pageNum - 1) * size)).SetLimit(int64(size))

	// 如果Page结构体中包含排序字段，则设置排序条件
	if page.Sort != nil {
		opts.SetSort(page.Sort)
	}

	cursor, err := d.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var entities []*T
	if err = cursor.All(ctx, &entities); err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}

func (d *MongoDaoImpl[T]) Count(ctx context.Context, condition *T) (int64, error) {
	return d.coll.CountDocuments(ctx, condition)
}

func (d *MongoDaoImpl[T]) CountByMap(ctx context.Context, conditions map[string]interface{}) (int64, error) {
	return d.coll.CountDocuments(ctx, conditions)
}

func (d *MongoDaoImpl[T]) Exists(ctx context.Context, condition *T) (bool, error) {
	count, err := d.Count(ctx, condition)
	return count > 0, err
}

func (d *MongoDaoImpl[T]) ExistsByMap(ctx context.Context, conditions map[string]interface{}) (bool, error) {
	count, err := d.CountByMap(ctx, conditions)
	return count > 0, err
}

// find 通用查询方法
func (d *MongoDaoImpl[T]) find(ctx context.Context, filter interface{}) ([]*T, error) {
	cursor, err := d.coll.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var entities []*T
	if err = cursor.All(ctx, &entities); err != nil {
		return nil, err
	}

	return entities, nil
}

// toObjectId 转换为MongoDB的ObjectId
func toObjectId(id interface{}) primitive.ObjectID {
	switch v := id.(type) {
	case primitive.ObjectID:
		return v
	case string:
		if objectId, err := primitive.ObjectIDFromHex(v); err == nil {
			return objectId
		}
	}
	return primitive.NilObjectID
}
