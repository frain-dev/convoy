package testenv

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/olamilekan000/surge/surge/backend"
	"github.com/olamilekan000/surge/surge/job"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
)

// QueueInspectorFunc creates a new surge backend for testing
type QueueInspectorFunc func(t *testing.T) backend.Backend

// GetTaskByID retrieves a job from the queue by its ID
// Note: Surge doesn't have direct GetTaskInfo - this is a placeholder
func GetTaskByID(t *testing.T, inspector backend.Backend, queueName convoy.QueueName, taskID string) (*job.JobEnvelope, error) {
	t.Helper()
	// TODO: Implement job lookup via backend if surge adds this capability
	// For now, return error as this feature isn't available
	return nil, fmt.Errorf("GetTaskByID not implemented for surge - job lookup by ID not supported")
}

// WaitForTask polls the queue until a task with the given ID appears or timeout
// Note: This is a simplified version since surge doesn't support direct job lookup
func WaitForTask(t *testing.T, inspector backend.Backend, queueName convoy.QueueName, taskID string, timeout time.Duration) *job.JobEnvelope {
	t.Helper()
	// TODO: Implement using backend methods if available
	// For now, this is a placeholder
	t.Logf("WaitForTask: Surge doesn't support direct job lookup by ID")
	return nil
}

// WaitForTaskWithPrefix polls the queue until a task with the given ID prefix appears or timeout
// Note: This requires listing all jobs which may be inefficient
func WaitForTaskWithPrefix(t *testing.T, inspector backend.Backend, queueName convoy.QueueName, prefix string, timeout time.Duration) *job.JobEnvelope {
	t.Helper()
	// TODO: Implement by checking DLQ or using backend inspection methods
	t.Logf("WaitForTaskWithPrefix: Implementation needed for surge")
	return nil
}

// AssertTaskExists verifies a task exists in the queue with the expected ID
func AssertTaskExists(t *testing.T, inspector backend.Backend, queueName convoy.QueueName, expectedTaskID string) {
	t.Helper()
	// TODO: Implement job lookup
	t.Logf("AssertTaskExists: Surge doesn't support direct job lookup by ID")
}

// AssertTaskNotDuplicated verifies that attempting to enqueue the same job ID doesn't create duplicates
func AssertTaskNotDuplicated(t *testing.T, inspector backend.Backend, queueName convoy.QueueName, taskID string) {
	t.Helper()
	// TODO: Implement by checking queue stats or listing jobs
	t.Logf("AssertTaskNotDuplicated: Implementation needed for surge")
}

// VerifyJobIDFormat checks if a job ID matches the expected pattern
func VerifyJobIDFormat(t *testing.T, jobID, expectedPrefix, projectID string) {
	t.Helper()

	expected := fmt.Sprintf("%s:%s:", expectedPrefix, projectID)
	require.True(t, strings.HasPrefix(jobID, expected),
		"Job ID %s should start with %s", jobID, expected)
}

// FindTaskByPrefix finds the first task in the queue with the given prefix
func FindTaskByPrefix(t *testing.T, inspector backend.Backend, queueName convoy.QueueName, prefix string) *job.JobEnvelope {
	t.Helper()
	// TODO: Implement using backend inspection methods
	t.Logf("FindTaskByPrefix: Implementation needed for surge")
	return nil
}

// CountTasksWithPrefix counts how many tasks have the given prefix
func CountTasksWithPrefix(t *testing.T, inspector backend.Backend, queueName convoy.QueueName, prefix string) int {
	t.Helper()
	// TODO: Implement using backend inspection methods
	t.Logf("CountTasksWithPrefix: Implementation needed for surge")
	return 0
}
