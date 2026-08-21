package app

import (
	"context"
	"strings"
	"testing"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTxNestingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowInstanceModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

// 验证 mvc.ExtractDB(ctx, base.DB).Transaction 在 ctx 已携带事务时复用该事务，
// 而不是另开一个——这是 Task 3 事务透传不死锁的机制前提。
//
// 注意：SQLite 无行锁且静默忽略 FOR UPDATE，本用例只能证明「机制上复用了同一
// 事务」，不能证明真实数据库上不死锁。后者见 Task 7 的真机验证。
func TestExtractDBTransactionReusesOuterTx(t *testing.T) {
	db := newTxNestingTestDB(t)
	prev := base.DB
	base.DB = db
	t.Cleanup(func() { base.DB = prev })

	inst := &model.WorkflowInstanceModel{
		DefID: 1, DefVersion: 1, Status: model.InstanceStatusRunning,
		InitialState: `{}`, CurrentState: `{}`, ActiveNodeIDs: `[]`,
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	ctx := context.Background()
	var innerTxPtr, outerTxPtr *gorm.DB

	err := mvc.ExtractDB(ctx, base.DB).Transaction(func(outer *gorm.DB) error {
		outerTxPtr = outer
		txCtx := mvc.WithTxToContext(ctx, outer)
		// 模拟 Task 3 中 triggerNode 递归回到某个事务入口的情形
		return mvc.ExtractDB(txCtx, base.DB).Transaction(func(inner *gorm.DB) error {
			innerTxPtr = inner
			return inner.Model(&model.WorkflowInstanceModel{}).
				Where("id = ?", inst.ID).
				Update("status", model.InstanceStatusWaiting).Error
		})
	})
	if err != nil {
		t.Fatalf("nested transaction failed: %v", err)
	}
	if outerTxPtr == nil || innerTxPtr == nil {
		t.Fatal("transaction callbacks did not run")
	}
	if outerTxPtr.Statement.ConnPool != innerTxPtr.Statement.ConnPool {
		t.Fatal("内层事务未复用外层连接池 —— 说明另开了事务，真机上会死锁")
	}

	var got model.WorkflowInstanceModel
	if err := db.Where("id = ?", inst.ID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != model.InstanceStatusWaiting {
		t.Fatalf("status = %s, want %s（嵌套事务的写入未提交）", got.Status, model.InstanceStatusWaiting)
	}
}

// 无外层事务时，ExtractDB 必须退化为 base.DB，行为与改造前完全一致。
func TestExtractDBTransactionWithoutOuterTxStillCommits(t *testing.T) {
	db := newTxNestingTestDB(t)
	prev := base.DB
	base.DB = db
	t.Cleanup(func() { base.DB = prev })

	inst := &model.WorkflowInstanceModel{
		DefID: 1, DefVersion: 1, Status: model.InstanceStatusRunning,
		InitialState: `{}`, CurrentState: `{}`, ActiveNodeIDs: `[]`,
	}
	if err := db.Create(inst).Error; err != nil {
		t.Fatalf("create instance: %v", err)
	}

	err := mvc.ExtractDB(context.Background(), base.DB).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&model.WorkflowInstanceModel{}).
			Where("id = ?", inst.ID).
			Update("status", model.InstanceStatusCompleted).Error
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	var got model.WorkflowInstanceModel
	if err := db.Where("id = ?", inst.ID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != model.InstanceStatusCompleted {
		t.Fatalf("status = %s, want %s", got.Status, model.InstanceStatusCompleted)
	}
}
