package dao

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xsxdot/aio/system/workflow/internal/model"
	"github.com/xsxdot/gokit/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAppliedCallbackTestDao(t *testing.T) (*WorkflowAppliedCallbackDao, *gorm.DB) {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.WorkflowAppliedCallbackModel{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return NewWorkflowAppliedCallbackDao(db, logger.GetLogger()), db
}

// 首次登记返回 true，同 jobID 再次登记返回 false —— 这是整个 outbox 重试安全的基石。
func TestMarkAppliedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	d, db := newAppliedCallbackTestDao(t)

	applied, err := d.MarkApplied(ctx, db, 1001, 77, "E3_MicroShotInspect")
	if err != nil {
		t.Fatalf("first MarkApplied: %v", err)
	}
	if !applied {
		t.Fatal("first MarkApplied = false, want true")
	}

	applied, err = d.MarkApplied(ctx, db, 1001, 77, "E3_MicroShotInspect")
	if err != nil {
		t.Fatalf("second MarkApplied: %v", err)
	}
	if applied {
		t.Fatal("second MarkApplied = true, want false (duplicate must be rejected)")
	}

	var count int64
	if err := db.Model(&model.WorkflowAppliedCallbackModel{}).Where("job_id = ?", 1001).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

// 不同 jobID 互不影响
func TestMarkAppliedDistinctJobIDs(t *testing.T) {
	ctx := context.Background()
	d, db := newAppliedCallbackTestDao(t)

	for _, id := range []int64{1, 2, 3} {
		applied, err := d.MarkApplied(ctx, db, id, 77, "N")
		if err != nil {
			t.Fatalf("MarkApplied(%d): %v", id, err)
		}
		if !applied {
			t.Fatalf("MarkApplied(%d) = false, want true", id)
		}
	}
}

// 清理必须硬删：软删除的行仍占唯一索引，会让该 jobID 永久无法再登记。
func TestCleanupBeforeHardDeletesSoUniqueKeyIsFreed(t *testing.T) {
	ctx := context.Background()
	d, db := newAppliedCallbackTestDao(t)

	if _, err := d.MarkApplied(ctx, db, 2002, 88, "N"); err != nil {
		t.Fatalf("MarkApplied: %v", err)
	}
	// 把创建时间拨到过去，使其落入清理窗口
	if err := db.Model(&model.WorkflowAppliedCallbackModel{}).
		Where("job_id = ?", 2002).
		Update("created_at", time.Now().Add(-48*time.Hour)).Error; err != nil {
		t.Fatalf("backdate: %v", err)
	}

	n, err := d.CleanupBefore(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("CleanupBefore: %v", err)
	}
	if n != 1 {
		t.Fatalf("CleanupBefore deleted %d rows, want 1", n)
	}

	var raw int64
	if err := db.Unscoped().Model(&model.WorkflowAppliedCallbackModel{}).
		Where("job_id = ?", 2002).Count(&raw).Error; err != nil {
		t.Fatalf("raw count: %v", err)
	}
	if raw != 0 {
		t.Fatalf("row still present after cleanup (raw count = %d) — 说明用了软删除，唯一索引未释放", raw)
	}

	// 清理后同 jobID 必须能重新登记
	applied, err := d.MarkApplied(ctx, db, 2002, 88, "N")
	if err != nil {
		t.Fatalf("re-MarkApplied: %v", err)
	}
	if !applied {
		t.Fatal("re-MarkApplied = false, want true — 唯一索引未被清理释放")
	}
}
