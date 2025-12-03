package testenv

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
)

// QueueInspectorFunc creates a new asynq inspector for testing
type QueueInspectorFunc func(t *testing.T) *asynq.Inspector

// GetTaskByID retrieves a task from the queue by its ID
func GetTaskByID(t *testing.T, inspector *asynq.Inspector, queueName convoy.QueueName, taskID string) (*asynq.TaskInfo, error) {
	t.Helper()
	return inspector.GetTaskInfo(string(queueName), taskID)
}

// WaitForTask polls the queue until a task with the given ID appears or timeout
func WaitForTask(t *testing.T, inspector *asynq.Inspector, queueName convoy.QueueName, taskID string, timeout time.Duration) *asynq.TaskInfo {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		taskInfo, err := inspector.GetTaskInfo(string(queueName), taskID)
		if err == nil {
			return taskInfo
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Task %s not found in queue %s within %v", taskID, queueName, timeout)
	return nil
}

// WaitForTaskWithPrefix polls the queue until a task with the given ID prefix appears or timeout
func WaitForTaskWithPrefix(t *testing.T, inspector *asynq.Inspector, queueName convoy.QueueName, prefix string, timeout time.Duration) *asynq.TaskInfo {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		tasks, err := inspector.ListPendingTasks(string(queueName))
		if err == nil {
			for _, task := range tasks {
				if strings.HasPrefix(task.ID, prefix) {
					return task
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Task with prefix %s not found in queue %s within %v", prefix, queueName, timeout)
	return nil
}

// AssertTaskExists verifies a task exists in the queue with the expected ID
func AssertTaskExists(t *testing.T, inspector *asynq.Inspector, queueName convoy.QueueName, expectedTaskID string) {
	t.Helper()

	taskInfo, err := inspector.GetTaskInfo(string(queueName), expectedTaskID)
	require.NoError(t, err, "Task should exist in queue")
	require.Equal(t, expectedTaskID, taskInfo.ID, "Task ID should match")
}

// AssertTaskNotDuplicated verifies that attempting to enqueue the same job ID doesn't create duplicates
func AssertTaskNotDuplicated(t *testing.T, inspector *asynq.Inspector, queueName convoy.QueueName, taskID string) {
	t.Helper()

	// Get all tasks in queue
	tasks, err := inspector.ListPendingTasks(string(queueName))
	require.NoError(t, err)

	count := 0
	for _, task := range tasks {
		if task.ID == taskID {
			count++
		}
	}

	require.LessOrEqual(t, count, 1, "Task should not be duplicated in queue")
}

// VerifyJobIDFormat checks if a job ID matches the expected pattern
func VerifyJobIDFormat(t *testing.T, jobID, expectedPrefix, projectID string) {
	t.Helper()

	expected := fmt.Sprintf("%s:%s:", expectedPrefix, projectID)
	require.True(t, strings.HasPrefix(jobID, expected),
		"Job ID %s should start with %s", jobID, expected)
}

// FindTaskByPrefix finds the first task in the queue with the given prefix
func FindTaskByPrefix(t *testing.T, inspector *asynq.Inspector, queueName convoy.QueueName, prefix string) *asynq.TaskInfo {
	t.Helper()

	tasks, err := inspector.ListPendingTasks(string(queueName))
	require.NoError(t, err)

	for _, task := range tasks {
		if strings.HasPrefix(task.ID, prefix) {
			return task
		}
	}

	return nil
}

// CountTasksWithPrefix counts how many tasks have the given prefix
func CountTasksWithPrefix(t *testing.T, inspector *asynq.Inspector, queueName convoy.QueueName, prefix string) int {
	t.Helper()

	tasks, err := inspector.ListPendingTasks(string(queueName))
	require.NoError(t, err)

	count := 0
	for _, task := range tasks {
		if strings.HasPrefix(task.ID, prefix) {
			count++
		}
	}

	return count
}
