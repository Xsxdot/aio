package mvc

import (
	"context"
	"github.com/xsxdot/aio/pkg/core/model/common"
	"github.com/xsxdot/aio/pkg/core/result"

	"github.com/gofiber/fiber/v2"
)

// IBaseController 基础控制器接口
type IBaseController[T any] interface {
	// Create 创建实体
	Create(c *fiber.Ctx) error
	// CreateBatch 批量创建实体
	CreateBatch(c *fiber.Ctx) error
	// DeleteById 根据ID删除
	DeleteById(c *fiber.Ctx) error
	// DeleteByIds 根据ID批量删除
	DeleteByIds(c *fiber.Ctx) error
	// UpdateById 根据ID更新
	UpdateById(c *fiber.Ctx) error
	// FindById 根据ID查询
	FindById(c *fiber.Ctx) error
	// FindPage 分页查询
	FindPage(c *fiber.Ctx) error
	// FindByUserId 根据用户ID查询
	FindByUserId(c *fiber.Ctx) error
}

// BaseControllerImpl 基础控制器实现
type BaseControllerImpl[T any] struct {
	S IBaseService[T]
}

// NewBaseController 创建基础控制器实例
func NewBaseController[T any](service IBaseService[T]) IBaseController[T] {
	return &BaseControllerImpl[T]{
		S: service,
	}
}

func (ctrl *BaseControllerImpl[T]) Create(c *fiber.Ctx) error {
	var entity T
	if err := c.BodyParser(&entity); err != nil {
		return err
	}

	ctx := context.Background()
	err := ctrl.S.Create(ctx, &entity)

	return result.Once(c, true, err)
}

func (ctrl *BaseControllerImpl[T]) CreateBatch(c *fiber.Ctx) error {
	var entities []*T
	if err := c.BodyParser(&entities); err != nil {
		return err
	}

	ctx := context.Background()
	err := ctrl.S.CreateBatch(ctx, entities)
	return result.Once(c, true, err)
}

func (ctrl *BaseControllerImpl[T]) DeleteById(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := context.Background()

	err := ctrl.S.DeleteById(ctx, id)
	return result.Once(c, true, err)
}

func (ctrl *BaseControllerImpl[T]) DeleteByIds(c *fiber.Ctx) error {
	var ids []interface{}
	if err := c.BodyParser(&ids); err != nil {
		return err
	}

	ctx := context.Background()
	rows, err := ctrl.S.DeleteByIds(ctx, ids)
	return result.Once(c, fiber.Map{"rows": rows}, err)
}

func (ctrl *BaseControllerImpl[T]) UpdateById(c *fiber.Ctx) error {
	id := c.Params("id")
	var entity T
	if err := c.BodyParser(&entity); err != nil {
		return err
	}

	ctx := context.Background()
	rows, err := ctrl.S.UpdateById(ctx, id, &entity)
	return result.Once(c, fiber.Map{"rows": rows}, err)
}

func (ctrl *BaseControllerImpl[T]) FindById(c *fiber.Ctx) error {
	id := c.Params("id")
	ctx := context.Background()

	entity, err := ctrl.S.FindById(ctx, id)
	return result.Once(c, entity, err)
}

func (ctrl *BaseControllerImpl[T]) FindPage(c *fiber.Ctx) error {
	var page Page
	if err := c.QueryParser(&page); err != nil {
		return err
	}

	var condition T
	if err := c.BodyParser(&condition); err != nil {
		return err
	}

	ctx := context.Background()
	entities, total, err := ctrl.S.FindPage(ctx, &page, &condition)

	return result.Once(c, common.PageReturn{
		Total:   total,
		Content: entities,
	}, err)
}

func (ctrl *BaseControllerImpl[T]) FindByUserId(c *fiber.Ctx) error {
	userId := c.Params("userId")
	ctx := context.Background()

	entities, err := ctrl.S.FindByUserId(ctx, userId)
	return result.Once(c, entities, err)
}
