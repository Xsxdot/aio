package grpc

import (
	"context"
	"testing"

	pb "github.com/xsxdot/aio/system/executor/api/proto"
	"github.com/xsxdot/gokit/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestExecutorServiceAcquireJobsRejectsEmptyEnv(t *testing.T) {
	svc := &ExecutorService{log: logger.GetLogger().WithEntryName("ExecutorService")}
	_, err := svc.AcquireJobs(context.Background(), &pb.AcquireJobsRequest{
		TargetService:  "tk-server",
		Methods:        []string{"method.a"},
		BaseConsumerId: "worker-base",
		Mode:           pb.AcquireJobsMode_ACQUIRE_JOBS_MODE_ONE_PER_METHOD,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, err = %v", status.Code(err), err)
	}
}

func TestExecutorServiceAcquireJobsRejectsInvalidMode(t *testing.T) {
	svc := &ExecutorService{log: logger.GetLogger().WithEntryName("ExecutorService")}
	_, err := svc.AcquireJobs(context.Background(), &pb.AcquireJobsRequest{
		Env:            "dev",
		TargetService:  "tk-server",
		Methods:        []string{"method.a"},
		BaseConsumerId: "worker-base",
		Mode:           pb.AcquireJobsMode_ACQUIRE_JOBS_MODE_UNSPECIFIED,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %v, err = %v", status.Code(err), err)
	}
}
