package dao

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xsxdot/aio/system/executor/internal/model"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openAcquireJobsDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	var (
		db  *gorm.DB
		err error
	)
	switch name {
	case "postgres":
		dsn := os.Getenv("AIO_EXECUTOR_TEST_POSTGRES_URL")
		if dsn == "" {
			t.Skip("AIO_EXECUTOR_TEST_POSTGRES_URL is not set")
		}
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	case "mysql":
		dsn := os.Getenv("AIO_EXECUTOR_TEST_MYSQL_DSN")
		if dsn == "" {
			t.Skip("AIO_EXECUTOR_TEST_MYSQL_DSN is not set")
		}
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		t.Fatalf("unsupported dialect %q", name)
	}
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	if err := db.Migrator().DropTable(&model.ExecutorJobAttemptModel{}, &model.ExecutorJobModel{}); err != nil {
		t.Fatalf("drop executor tables: %v", err)
	}
	if err := db.AutoMigrate(&model.ExecutorJobModel{}, &model.ExecutorJobAttemptModel{}); err != nil {
		t.Fatalf("migrate executor tables: %v", err)
	}
	return db
}

func forEachAcquireJobsDialect(t *testing.T, fn func(t *testing.T, db *gorm.DB)) {
	t.Helper()
	for _, name := range []string{"postgres", "mysql"} {
		name := name
		t.Run(name, func(t *testing.T) {
			fn(t, openAcquireJobsDB(t, name))
		})
	}
}

func seedAcquireJob(t *testing.T, db *gorm.DB, method string, priority int32, sequenceKey string) *model.ExecutorJobModel {
	t.Helper()
	nextRunAt := time.Now().Add(-time.Minute)
	job := &model.ExecutorJobModel{
		Env:           "dev",
		TargetService: "tk-server",
		Method:        method,
		ArgsJSON:      fmt.Sprintf(`{"method":%q}`, method),
		Status:        model.JobStatusPending,
		Priority:      priority,
		NextRunAt:     &nextRunAt,
		MaxAttempts:   3,
		DedupKey:      fmt.Sprintf("%s-%d-%d", method, priority, time.Now().UnixNano()),
		SequenceKey:   sequenceKey,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}
	return job
}

func TestExecutorJobDAOAcquireJobsOnePerMethodReturnsOnePerMethod(t *testing.T) {
	forEachAcquireJobsDialect(t, func(t *testing.T, db *gorm.DB) {
		d := NewExecutorJobDAOWithDB(db)
		seedAcquireJob(t, db, "method.a", 10, "")
		seedAcquireJob(t, db, "method.b", 5, "")

		leases, err := d.AcquireJobs(context.Background(), AcquireJobsInput{
			Env:           "dev",
			TargetService: "tk-server",
			Methods:       []string{"method.a", "method.b"},
			MethodSlots: []MethodSlot{
				{Method: "method.a", ConsumerID: "worker-base-m-method.a"},
				{Method: "method.b", ConsumerID: "worker-base-m-method.b"},
			},
			LeaseDuration: 60,
			Mode:          AcquireJobsModeOnePerMethod,
		})
		if err != nil {
			t.Fatalf("AcquireJobs: %v", err)
		}
		if len(leases) != 2 {
			t.Fatalf("leases len = %d, want 2", len(leases))
		}
		got := map[string]string{}
		for _, lease := range leases {
			got[lease.Job.Method] = lease.ConsumerID
		}
		if got["method.a"] != "worker-base-m-method.a" || got["method.b"] != "worker-base-m-method.b" {
			t.Fatalf("method consumer mapping = %#v", got)
		}
	})
}

func TestExecutorJobDAOAcquireJobsOnePerMethodSkipsBusyMethodSlot(t *testing.T) {
	forEachAcquireJobsDialect(t, func(t *testing.T, db *gorm.DB) {
		d := NewExecutorJobDAOWithDB(db)
		seedAcquireJob(t, db, "method.a", 10, "")
		seedAcquireJob(t, db, "method.b", 5, "")
		future := time.Now().Add(time.Minute)
		if err := db.Create(&model.ExecutorJobModel{
			Env:           "dev",
			TargetService: "tk-server",
			Method:        "method.a",
			ArgsJSON:      `{}`,
			Status:        model.JobStatusRunning,
			LeaseOwner:    "worker-base-m-method.a",
			LeaseUntil:    &future,
			MaxAttempts:   3,
			DedupKey:      "busy-method-a",
		}).Error; err != nil {
			t.Fatal(err)
		}

		leases, err := d.AcquireJobs(context.Background(), AcquireJobsInput{
			Env:           "dev",
			TargetService: "tk-server",
			Methods:       []string{"method.a", "method.b"},
			MethodSlots: []MethodSlot{
				{Method: "method.a", ConsumerID: "worker-base-m-method.a"},
				{Method: "method.b", ConsumerID: "worker-base-m-method.b"},
			},
			LeaseDuration: 60,
			Mode:          AcquireJobsModeOnePerMethod,
		})
		if err != nil {
			t.Fatalf("AcquireJobs: %v", err)
		}
		if len(leases) != 1 || leases[0].Job.Method != "method.b" || leases[0].ConsumerID != "worker-base-m-method.b" {
			t.Fatalf("leases = %#v, want only method.b on worker-base-m-method.b", leases)
		}
	})
}

func TestExecutorJobDAOAcquireJobsFillSlotsAllowsSameMethod(t *testing.T) {
	forEachAcquireJobsDialect(t, func(t *testing.T, db *gorm.DB) {
		d := NewExecutorJobDAOWithDB(db)
		seedAcquireJob(t, db, "method.a", 10, "")
		seedAcquireJob(t, db, "method.a", 9, "")
		seedAcquireJob(t, db, "method.b", 1, "")

		leases, err := d.AcquireJobs(context.Background(), AcquireJobsInput{
			Env:           "dev",
			TargetService: "tk-server",
			Methods:       []string{"method.a", "method.b"},
			MethodSlots: []MethodSlot{
				{ConsumerID: "slot-0"},
				{ConsumerID: "slot-1"},
			},
			LeaseDuration: 60,
			Mode:          AcquireJobsModeFillSlots,
		})
		if err != nil {
			t.Fatalf("AcquireJobs: %v", err)
		}
		if len(leases) != 2 {
			t.Fatalf("leases len = %d, want 2", len(leases))
		}
		if leases[0].Job.Method != "method.a" || leases[1].Job.Method != "method.a" {
			t.Fatalf("fill slots should take two highest priority method.a jobs, got %s and %s", leases[0].Job.Method, leases[1].Job.Method)
		}
	})
}

func TestExecutorJobDAOAcquireJobsFillSlotsDoesNotBusyFilterSlots(t *testing.T) {
	forEachAcquireJobsDialect(t, func(t *testing.T, db *gorm.DB) {
		d := NewExecutorJobDAOWithDB(db)
		seedAcquireJob(t, db, "method.a", 10, "")
		seedAcquireJob(t, db, "method.a", 9, "")
		future := time.Now().Add(time.Minute)
		if err := db.Create(&model.ExecutorJobModel{
			Env:           "dev",
			TargetService: "tk-server",
			Method:        "method.busy",
			ArgsJSON:      `{}`,
			Status:        model.JobStatusRunning,
			LeaseOwner:    "slot-0",
			LeaseUntil:    &future,
			MaxAttempts:   3,
			DedupKey:      "busy-slot-0",
		}).Error; err != nil {
			t.Fatal(err)
		}

		leases, err := d.AcquireJobs(context.Background(), AcquireJobsInput{
			Env:           "dev",
			TargetService: "tk-server",
			Methods:       []string{"method.a"},
			MethodSlots: []MethodSlot{
				{ConsumerID: "slot-0"},
				{ConsumerID: "slot-1"},
			},
			LeaseDuration: 60,
			Mode:          AcquireJobsModeFillSlots,
		})
		if err != nil {
			t.Fatalf("AcquireJobs: %v", err)
		}
		if len(leases) != 2 {
			t.Fatalf("leases len = %d, want 2 because FILL_SLOTS trusts SDK free slots", len(leases))
		}
		if leases[0].ConsumerID != "slot-0" || leases[1].ConsumerID != "slot-1" {
			t.Fatalf("consumer order = %s,%s; want slot-0,slot-1", leases[0].ConsumerID, leases[1].ConsumerID)
		}
	})
}

func TestExecutorJobDAOAcquireJobsRespectsSequenceKey(t *testing.T) {
	forEachAcquireJobsDialect(t, func(t *testing.T, db *gorm.DB) {
		d := NewExecutorJobDAOWithDB(db)
		seedAcquireJob(t, db, "method.a", 10, "seq-1")
		future := time.Now().Add(time.Minute)
		if err := db.Create(&model.ExecutorJobModel{
			Env:           "dev",
			TargetService: "tk-server",
			Method:        "method.a",
			ArgsJSON:      `{}`,
			Status:        model.JobStatusRunning,
			LeaseOwner:    "other-worker",
			LeaseUntil:    &future,
			MaxAttempts:   3,
			DedupKey:      "running-seq",
			SequenceKey:   "seq-1",
		}).Error; err != nil {
			t.Fatal(err)
		}

		leases, err := d.AcquireJobs(context.Background(), AcquireJobsInput{
			Env:           "dev",
			TargetService: "tk-server",
			Methods:       []string{"method.a"},
			MethodSlots: []MethodSlot{
				{Method: "method.a", ConsumerID: "worker-base-m-method.a"},
			},
			LeaseDuration: 60,
			Mode:          AcquireJobsModeOnePerMethod,
		})
		if err != nil {
			t.Fatalf("AcquireJobs: %v", err)
		}
		if len(leases) != 0 {
			t.Fatalf("leases len = %d, want 0 while sequence key is blocked", len(leases))
		}
	})
}

func TestExecutorJobDAOAcquireJobsCreatesAttemptsWithIncrementedAttemptNo(t *testing.T) {
	forEachAcquireJobsDialect(t, func(t *testing.T, db *gorm.DB) {
		d := NewExecutorJobDAOWithDB(db)
		job := seedAcquireJob(t, db, "method.a", 10, "")

		leases, err := d.AcquireJobs(context.Background(), AcquireJobsInput{
			Env:           "dev",
			TargetService: "tk-server",
			Methods:       []string{"method.a"},
			MethodSlots: []MethodSlot{
				{Method: "method.a", ConsumerID: "worker-base-m-method.a"},
			},
			LeaseDuration: 60,
			Mode:          AcquireJobsModeOnePerMethod,
		})
		if err != nil {
			t.Fatalf("AcquireJobs: %v", err)
		}
		if len(leases) != 1 {
			t.Fatalf("leases len = %d, want 1", len(leases))
		}

		var attempts []model.ExecutorJobAttemptModel
		if err := db.Where("job_id = ?", job.ID).Find(&attempts).Error; err != nil {
			t.Fatalf("find attempts: %v", err)
		}
		if len(attempts) != 1 {
			t.Fatalf("attempt len = %d, want 1", len(attempts))
		}
		if attempts[0].JobID != job.ID {
			t.Fatalf("attempt job_id = %d, want %d", attempts[0].JobID, job.ID)
		}
		if attempts[0].AttemptNo != leases[0].AttemptNo || attempts[0].AttemptNo != leases[0].Job.Attempts {
			t.Fatalf("attempt_no = %d, lease attempt=%d job attempts=%d", attempts[0].AttemptNo, leases[0].AttemptNo, leases[0].Job.Attempts)
		}
		if attempts[0].WorkerID != "worker-base-m-method.a" {
			t.Fatalf("worker_id = %q", attempts[0].WorkerID)
		}
	})
}

func TestPostgresValuesBuildersCastComparableColumns(t *testing.T) {
	methodSlotSQL, methodSlotArgs := buildMethodSlotValues([]MethodSlot{{Method: "method.a", ConsumerID: "slot-0"}})
	if !strings.Contains(methodSlotSQL, "?::text, ?::text, ?::bigint") {
		t.Fatalf("method slot values SQL = %q, want explicit text/text/bigint casts", methodSlotSQL)
	}
	if _, ok := methodSlotArgs[2].(int); !ok {
		t.Fatalf("method slot ord arg type = %T, want int", methodSlotArgs[2])
	}

	consumerSQL, consumerArgs := buildConsumerValues([]MethodSlot{{ConsumerID: "slot-0"}})
	if !strings.Contains(consumerSQL, "?::text, ?::bigint") {
		t.Fatalf("consumer values SQL = %q, want explicit text/bigint casts", consumerSQL)
	}
	if _, ok := consumerArgs[1].(int); !ok {
		t.Fatalf("consumer ord arg type = %T, want int", consumerArgs[1])
	}

	methodSQL, methodArgs := buildMethodValues([]string{"method.a"})
	if !strings.Contains(methodSQL, "?::text") {
		t.Fatalf("method values SQL = %q, want explicit text cast", methodSQL)
	}
	if _, ok := methodArgs[0].(string); !ok {
		t.Fatalf("method arg type = %T, want string", methodArgs[0])
	}
}
