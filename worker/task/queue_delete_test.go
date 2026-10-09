package task

import (
	"testing"

	pgqueue "github.com/frain-dev/convoy/queue/postgres"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
)

// TestBothProvidersRemoveQueuedJobs guards the silent branch in
// removeQueuedJobs: a queue that does not satisfy jobRemover returns nil and the
// retry path re-enqueues on top of rows it meant to clear, with no error
// anywhere. A compile-time assertion is the cheap way to keep that from
// happening by omission when either provider's method set changes.
func TestBothProvidersRemoveQueuedJobs(t *testing.T) {
	var _ jobRemover = (*pgqueue.PostgresQueue)(nil)
	var _ jobRemover = (*redisqueue.RedisQueue)(nil)
}
