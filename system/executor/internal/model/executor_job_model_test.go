package model

import "testing"

func TestExecutorJobAttemptJobIDUsesInt64(t *testing.T) {
	var attempt ExecutorJobAttemptModel
	var _ int64 = attempt.JobID
}
