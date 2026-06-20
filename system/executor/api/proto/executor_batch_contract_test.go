package proto

import "testing"

func TestAcquireJobsProtoContract(t *testing.T) {
	req := &AcquireJobsRequest{
		Env:            "dev",
		TargetService:  "tk-server",
		Methods:        []string{"method.a", "method.b"},
		BaseConsumerId: "worker-base",
		LeaseDuration:  60,
		Mode:           AcquireJobsMode_ACQUIRE_JOBS_MODE_ONE_PER_METHOD,
	}
	if req.GetBaseConsumerId() != "worker-base" {
		t.Fatalf("base_consumer_id = %q", req.GetBaseConsumerId())
	}
	if req.GetMode() != AcquireJobsMode_ACQUIRE_JOBS_MODE_ONE_PER_METHOD {
		t.Fatalf("mode = %v", req.GetMode())
	}

	resp := &AcquireJobsResponse{Jobs: []*AcquiredJobItem{{
		JobId:         42,
		AttemptNo:     1,
		Env:           "dev",
		TargetService: "tk-server",
		Method:        "method.a",
		ArgsJson:      `{"ok":true}`,
		LeaseUntil:    123456,
		ConsumerId:    "worker-base-m-method.a",
	}}}
	if got := resp.GetJobs()[0].GetConsumerId(); got != "worker-base-m-method.a" {
		t.Fatalf("consumer_id = %q", got)
	}
	var _ int64 = resp.GetJobs()[0].GetJobId()
}
