package service

import (
	"context"
	"testing"
	"time"

	"github.com/xsxdot/aio/base"
	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/api/dto"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"github.com/xsxdot/gokit/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func init() {
	base.Logger = logger.GetLogger()
}

func TestSubmitJob_ReusesDedupForTerminalStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ExecutorJobModel{}); err != nil {
		t.Fatal(err)
	}

	d := dao.NewExecutorJobDAOWithDB(db)
	s := &ExecutorJobService{dao: d, handlers: make(map[string]callback.JobCompletionHandler)}

	ctx := context.Background()
	dedup := "wf_42_node_H_VectorIngestion"
	env := "dev"
	next := time.Now()

	j := &model.ExecutorJobModel{
		Env:              env,
		TargetService:    "author",
		Method:           "genesis_vector_ingestion",
		ArgsJSON:         `{"old":true}`,
		Status:           model.JobStatusFailed,
		NextRunAt:        &next,
		MaxAttempts:      3,
		Attempts:         3,
		DedupKey:         dedup,
		RetryBackoffType: model.RetryBackoffExponential,
	}
	if err := db.Create(j).Error; err != nil {
		t.Fatal(err)
	}
	id1 := uint64(j.ID)

	idOut, err := s.SubmitJob(ctx, &dto.SubmitJobInput{
		Env:           env,
		TargetService: "author",
		Method:        "genesis_vector_ingestion",
		ArgsJSON:      `{"fresh":true}`,
		DedupKey:      dedup,
	})
	if err != nil {
		t.Fatal(err)
	}
	if idOut != id1 {
		t.Fatalf("job id: want %d got %d", id1, idOut)
	}

	var got model.ExecutorJobModel
	if err := db.First(&got, j.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != model.JobStatusPending {
		t.Fatalf("status want pending got %s", got.Status)
	}
	if got.ArgsJSON != `{"fresh":true}` {
		t.Fatalf("args: %s", got.ArgsJSON)
	}
	if got.Attempts != 0 {
		t.Fatalf("attempts want 0 got %d", got.Attempts)
	}
}

func TestSubmitJob_PendingDedupReturnsSameID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ExecutorJobModel{}); err != nil {
		t.Fatal(err)
	}

	d := dao.NewExecutorJobDAOWithDB(db)
	s := &ExecutorJobService{dao: d, handlers: make(map[string]callback.JobCompletionHandler)}

	ctx := context.Background()
	dedup := "wf_1_node_A"
	env := "dev"
	next := time.Now()

	j := &model.ExecutorJobModel{
		Env:              env,
		TargetService:    "author",
		Method:           "m",
		ArgsJSON:         `{"v":1}`,
		Status:           model.JobStatusPending,
		NextRunAt:        &next,
		MaxAttempts:      3,
		DedupKey:         dedup,
		RetryBackoffType: model.RetryBackoffExponential,
	}
	if err := db.Create(j).Error; err != nil {
		t.Fatal(err)
	}

	idOut, err := s.SubmitJob(ctx, &dto.SubmitJobInput{
		Env:           env,
		TargetService: "author",
		Method:        "m",
		ArgsJSON:      `{"v":2}`,
		DedupKey:      dedup,
	})
	if err != nil {
		t.Fatal(err)
	}
	if idOut != uint64(j.ID) {
		t.Fatalf("id: want %d got %d", j.ID, idOut)
	}

	var got model.ExecutorJobModel
	if err := db.First(&got, j.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.ArgsJSON != `{"v":1}` {
		t.Fatalf("pending dedup must not overwrite args, got %s", got.ArgsJSON)
	}
}

func TestSubmitJob_SucceededDedupUnchanged(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ExecutorJobModel{}); err != nil {
		t.Fatal(err)
	}

	d := dao.NewExecutorJobDAOWithDB(db)
	s := &ExecutorJobService{dao: d, handlers: make(map[string]callback.JobCompletionHandler)}

	ctx := context.Background()
	dedup := "wf_9_done"
	env := "dev"
	next := time.Now()

	j := &model.ExecutorJobModel{
		Env:              env,
		TargetService:    "author",
		Method:           "m",
		ArgsJSON:         `{"done":true}`,
		Status:           model.JobStatusSucceeded,
		NextRunAt:        &next,
		MaxAttempts:      3,
		DedupKey:         dedup,
		RetryBackoffType: model.RetryBackoffExponential,
	}
	if err := db.Create(j).Error; err != nil {
		t.Fatal(err)
	}

	idOut, err := s.SubmitJob(ctx, &dto.SubmitJobInput{
		Env:           env,
		TargetService: "author",
		Method:        "m",
		ArgsJSON:      `{"new":true}`,
		DedupKey:      dedup,
	})
	if err != nil {
		t.Fatal(err)
	}
	if idOut != uint64(j.ID) {
		t.Fatalf("id: want %d got %d", j.ID, idOut)
	}

	var got model.ExecutorJobModel
	if err := db.First(&got, j.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != model.JobStatusSucceeded || got.ArgsJSON != `{"done":true}` {
		t.Fatalf("succeeded job must stay unchanged: status=%s args=%s", got.Status, got.ArgsJSON)
	}
}
