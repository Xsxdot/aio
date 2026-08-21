package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
	errorc "github.com/xsxdot/gokit/err"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newAckOutboxTestService(t *testing.T) (*ExecutorJobService, *gorm.DB) {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ExecutorJobModel{}, &model.ExecutorJobAttemptModel{}); err != nil {
		t.Fatal(err)
	}
	d := dao.NewExecutorJobDAOWithDB(db)
	prev := base.DB
	base.DB = db
	t.Cleanup(func() { base.DB = prev })
	return &ExecutorJobService{
		dao:      d,
		handlers: make(map[string]callback.JobCompletionHandler),
		err:      errorc.NewErrorBuilder("ExecutorJobService"),
	}, db
}

// AckJob 成功且 job.Source 非空时，必须在同一事务内产生一条 outbox 回调任务。
func TestAckJobCreatesOutboxCallbackJob(t *testing.T) {
	ctx := context.Background()
	s, db := newAckOutboxTestService(t)

	now := time.Now()
	until := now.Add(time.Minute)
	job := &model.ExecutorJobModel{
		Env: "dev", TargetService: "tk-server", Method: "split_video",
		Status: model.JobStatusRunning, Attempts: 1, MaxAttempts: 3,
		DedupKey: "wf_1_node_A_1", Source: "workflow",
		CallbackData: `{"instance_id":1,"node_id":"A","env":"dev"}`,
		LeaseOwner:   "c-1", LeaseUntil: &until, NextRunAt: &now,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}

	if err := s.AckJob(ctx, uint64(job.ID), 1, "c-1",
		model.JobStatusSucceeded, "", `{"ok":true}`, 0, false, 0, ""); err != nil {
		t.Fatalf("AckJob: %v", err)
	}

	var outbox model.ExecutorJobModel
	wantDedup := callback.OutboxDedupKeyPrefix + strconv.FormatInt(job.ID, 10)
	if err := db.Where("dedup_key = ?", wantDedup).First(&outbox).Error; err != nil {
		t.Fatalf("outbox job 未创建 (dedup_key=%s): %v", wantDedup, err)
	}
	if outbox.TargetService != callback.InternalTargetService {
		t.Fatalf("TargetService = %q, want %q", outbox.TargetService, callback.InternalTargetService)
	}
	if outbox.Method != callback.MethodJobCompletedCallback {
		t.Fatalf("Method = %q, want %q", outbox.Method, callback.MethodJobCompletedCallback)
	}
	// Source 必须为空，否则这条回调任务自身 Ack 时又会生成回调，无限递归
	if outbox.Source != "" {
		t.Fatalf("outbox.Source = %q, want \"\" —— 非空会导致回调无限递归", outbox.Source)
	}

	var payload callback.CallbackPayload
	if err := json.Unmarshal([]byte(outbox.ArgsJSON), &payload); err != nil {
		t.Fatalf("解析 ArgsJSON: %v", err)
	}
	if payload.JobID != uint64(job.ID) || payload.Source != "workflow" {
		t.Fatalf("payload = %+v, want JobID=%d Source=workflow", payload, job.ID)
	}
	if payload.ResultJSON != `{"ok":true}` {
		t.Fatalf("payload.ResultJSON = %q", payload.ResultJSON)
	}
}

// job.Source 为空时不得产生 outbox 任务
func TestAckJobWithoutSourceCreatesNoOutbox(t *testing.T) {
	ctx := context.Background()
	s, db := newAckOutboxTestService(t)

	now := time.Now()
	until := now.Add(time.Minute)
	job := &model.ExecutorJobModel{
		Env: "dev", TargetService: "tk-server", Method: "split_video",
		Status: model.JobStatusRunning, Attempts: 1, MaxAttempts: 3,
		DedupKey: "plain_1", Source: "",
		LeaseOwner: "c-1", LeaseUntil: &until, NextRunAt: &now,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.AckJob(ctx, uint64(job.ID), 1, "c-1",
		model.JobStatusSucceeded, "", `{}`, 0, false, 0, ""); err != nil {
		t.Fatalf("AckJob: %v", err)
	}

	var count int64
	if err := db.Model(&model.ExecutorJobModel{}).
		Where("method = ?", callback.MethodJobCompletedCallback).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("outbox job count = %d, want 0", count)
	}
}
