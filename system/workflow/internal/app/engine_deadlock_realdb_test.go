// 职责：在真实 MySQL / PostgreSQL 上验证事务透传改造的两个判据——
// 嵌套加锁不死锁、幂等表的 ON CONFLICT DO NOTHING 跨方言行为一致。
//
// 边界：本文件只覆盖必须真机才能成立的判据。gorm.io/driver/sqlite 静默忽略
// FOR UPDATE（不报错、无行锁），因此其余测试在 SQLite 上的通过结果不构成
// 死锁方面的证据。未设置环境变量时整体 Skip，不影响常规 go test ./...。
package app

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/pkg/core/mvc"
	"github.com/xsxdot/aio/system/workflow/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// openRealDB 按方言打开一次性测试库并重建表结构。
//
// 注意：本函数会 DropTable，DSN 必须指向一次性实例，绝不能指向 dev/prod。
// 未设置对应环境变量时调用 t.Skip。
func openRealDB(t *testing.T, dialect string) *gorm.DB {
	t.Helper()
	var (
		db  *gorm.DB
		err error
	)
	switch dialect {
	case "postgres":
		dsn := os.Getenv("AIO_WORKFLOW_TEST_POSTGRES_URL")
		if dsn == "" {
			t.Skip("AIO_WORKFLOW_TEST_POSTGRES_URL is not set")
		}
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "mysql":
		dsn := os.Getenv("AIO_WORKFLOW_TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("AIO_WORKFLOW_TEST_MYSQL_DSN is not set")
		}
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported dialect %q", dialect)
	}
	if err != nil {
		t.Fatalf("open %s: %v", dialect, err)
	}
	if err := db.Migrator().DropTable(&model.WorkflowAppliedCallbackModel{}, &model.WorkflowInstanceModel{}); err != nil {
		t.Fatalf("drop tables: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowInstanceModel{}, &model.WorkflowAppliedCallbackModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// forEachRealDialect 对 postgres 与 mysql 各跑一遍 fn。
func forEachRealDialect(t *testing.T, fn func(t *testing.T, db *gorm.DB)) {
	t.Helper()
	for _, d := range []string{"postgres", "mysql"} {
		d := d
		t.Run(d, func(t *testing.T) { fn(t, openRealDB(t, d)) })
	}
}

// TestNestedLockingDoesNotDeadlockOnRealDB 核心判据：外层事务对实例行持有
// FOR UPDATE 时，内层通过 ExtractDB 重入同一实例行的加锁读必须立即成功
// （SavePoint 复用同一连接与事务），不得超时。
//
// 若任一事务入口被回退成 base.DB.Transaction，内层会另开连接等外层释放行锁、
// 外层又在等内层返回，本用例会挂到 15s 超时后失败。
func TestNestedLockingDoesNotDeadlockOnRealDB(t *testing.T) {
	forEachRealDialect(t, func(t *testing.T, db *gorm.DB) {
		prev := base.DB
		base.DB = db
		t.Cleanup(func() { base.DB = prev })

		inst := &model.WorkflowInstanceModel{
			DefID: 1, DefVersion: 1, Status: model.InstanceStatusRunning,
			InitialState:  `{"data":{},"_sys":{}}`,
			CurrentState:  `{"data":{},"_sys":{}}`,
			ActiveNodeIDs: `["A"]`,
		}
		if err := db.Create(inst).Error; err != nil {
			t.Fatalf("create instance: %v", err)
		}

		done := make(chan error, 1)
		go func() {
			ctx := context.Background()
			done <- mvc.ExtractDB(ctx, base.DB).Transaction(func(outer *gorm.DB) error {
				var locked model.WorkflowInstanceModel
				if err := outer.Clauses(clause.Locking{Strength: "UPDATE"}).
					Where("id = ?", inst.ID).First(&locked).Error; err != nil {
					return err
				}
				txCtx := mvc.WithTxToContext(ctx, outer)
				// 模拟 condition 递归重入 ReportNodeCompleted 的事务入口
				return mvc.ExtractDB(txCtx, base.DB).Transaction(func(inner *gorm.DB) error {
					var again model.WorkflowInstanceModel
					if err := inner.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("id = ?", inst.ID).First(&again).Error; err != nil {
						return err
					}
					return inner.Model(&model.WorkflowInstanceModel{}).
						Where("id = ?", inst.ID).
						Update("active_node_ids", `["B"]`).Error
				})
			})
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("嵌套加锁事务失败: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("嵌套加锁事务超时 —— 出现死锁，说明存在未改造为 ExtractDB 的事务入口")
		}

		var got model.WorkflowInstanceModel
		if err := db.Where("id = ?", inst.ID).First(&got).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if got.ActiveNodeIDs != `["B"]` {
			t.Fatalf("ActiveNodeIDs = %s, want [\"B\"]", got.ActiveNodeIDs)
		}
	})
}

// TestMarkAppliedOnConflictAcrossRealDialects 验证幂等表的 ON CONFLICT
// DO NOTHING 在两种真实方言上行为一致：重复插入静默跳过且 RowsAffected 为 0。
func TestMarkAppliedOnConflictAcrossRealDialects(t *testing.T) {
	forEachRealDialect(t, func(t *testing.T, db *gorm.DB) {
		rec := func() *model.WorkflowAppliedCallbackModel {
			return &model.WorkflowAppliedCallbackModel{JobID: 7777, InstanceID: 1, NodeID: "A"}
		}
		first := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}}, DoNothing: true,
		}).Create(rec())
		if first.Error != nil || first.RowsAffected != 1 {
			t.Fatalf("first insert: err=%v rows=%d, want rows=1", first.Error, first.RowsAffected)
		}
		second := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "job_id"}}, DoNothing: true,
		}).Create(rec())
		if second.Error != nil {
			t.Fatalf("second insert error: %v（期望静默跳过而非报错）", second.Error)
		}
		if second.RowsAffected != 0 {
			t.Fatalf("second insert rows = %d, want 0", second.RowsAffected)
		}
	})
}
