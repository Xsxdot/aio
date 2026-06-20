package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/xsxdot/aio/system/executor/api/callback"
	"github.com/xsxdot/aio/system/executor/internal/dao"
	"github.com/xsxdot/aio/system/executor/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExecutorJobServiceAcquireJobsRejectsInvalidRequests(t *testing.T) {
	s := &ExecutorJobService{}
	tests := []struct {
		name    string
		req     AcquireJobsRequest
		wantErr string
	}{
		{
			name:    "empty env",
			req:     AcquireJobsRequest{TargetService: "tk-server", Methods: []string{"method.a"}, BaseConsumerID: "worker", Mode: dao.AcquireJobsModeOnePerMethod},
			wantErr: "env 不能为空",
		},
		{
			name:    "empty target",
			req:     AcquireJobsRequest{Env: "dev", Methods: []string{"method.a"}, BaseConsumerID: "worker", Mode: dao.AcquireJobsModeOnePerMethod},
			wantErr: "targetService 不能为空",
		},
		{
			name:    "empty methods",
			req:     AcquireJobsRequest{Env: "dev", TargetService: "tk-server", BaseConsumerID: "worker", Mode: dao.AcquireJobsModeOnePerMethod},
			wantErr: "methods 不能为空",
		},
		{
			name:    "unspecified mode",
			req:     AcquireJobsRequest{Env: "dev", TargetService: "tk-server", Methods: []string{"method.a"}, BaseConsumerID: "worker"},
			wantErr: "mode 不合法",
		},
		{
			name:    "one per method missing base consumer",
			req:     AcquireJobsRequest{Env: "dev", TargetService: "tk-server", Methods: []string{"method.a"}, Mode: dao.AcquireJobsModeOnePerMethod},
			wantErr: "base_consumer_id 不能为空",
		},
		{
			name:    "fill slots missing consumers",
			req:     AcquireJobsRequest{Env: "dev", TargetService: "tk-server", Methods: []string{"method.a"}, Mode: dao.AcquireJobsModeFillSlots},
			wantErr: "consumer_ids 不能为空",
		},
		{
			name:    "duplicate methods",
			req:     AcquireJobsRequest{Env: "dev", TargetService: "tk-server", Methods: []string{"method.a", "method.a"}, BaseConsumerID: "worker", Mode: dao.AcquireJobsModeOnePerMethod},
			wantErr: "methods 不能重复",
		},
		{
			name:    "duplicate consumers",
			req:     AcquireJobsRequest{Env: "dev", TargetService: "tk-server", Methods: []string{"method.a"}, ConsumerIDs: []string{"slot-0", "slot-0"}, Mode: dao.AcquireJobsModeFillSlots},
			wantErr: "consumer_ids 不能重复",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.AcquireJobs(context.Background(), tt.req)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want contains %q", err, tt.wantErr)
			}
		})
	}
}

func TestExecutorJobServiceAcquireJobsDerivesOnePerMethodSlots(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:executor_acquire_jobs_service?mode=memory&cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrator().DropTable(&model.ExecutorJobAttemptModel{}, &model.ExecutorJobModel{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.ExecutorJobModel{}, &model.ExecutorJobAttemptModel{}); err != nil {
		t.Fatal(err)
	}
	nextRunAt := time.Now().Add(-time.Minute)
	job := &model.ExecutorJobModel{
		Env:           "dev",
		TargetService: "tk-server",
		Method:        "method.a",
		ArgsJSON:      `{}`,
		Status:        model.JobStatusPending,
		NextRunAt:     &nextRunAt,
		MaxAttempts:   3,
		DedupKey:      "service-derive-method-a",
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}

	d := dao.NewExecutorJobDAOWithDB(db)
	s := &ExecutorJobService{dao: d, handlers: make(map[string]callback.JobCompletionHandler)}
	leases, err := s.AcquireJobs(context.Background(), AcquireJobsRequest{
		Env:            "dev",
		TargetService:  "tk-server",
		Methods:        []string{"method.a"},
		BaseConsumerID: "worker-base",
		ConsumerIDs:    []string{"ignored"},
		LeaseDuration:  60,
		Mode:           dao.AcquireJobsModeOnePerMethod,
	})
	if err != nil {
		t.Fatalf("AcquireJobs: %v", err)
	}
	if len(leases) != 1 {
		t.Fatalf("leases len = %d, want 1", len(leases))
	}
	if leases[0].ConsumerID != "worker-base-m-method.a" {
		t.Fatalf("consumer_id = %q", leases[0].ConsumerID)
	}
	var got model.ExecutorJobModel
	if err := db.First(&got, job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.LeaseOwner != "worker-base-m-method.a" {
		t.Fatalf("lease_owner = %q", got.LeaseOwner)
	}
}
