package e2e

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/testenv"
	"github.com/frain-dev/convoy/worker"
)

// JobIDValidator stores captured job IDs for validation
type JobIDValidator struct {
	mu     sync.RWMutex
	jobIDs []string
	t      *testing.T
}

// NewJobIDValidator creates a new validator
func NewJobIDValidator(t *testing.T) *JobIDValidator {
	return &JobIDValidator{
		jobIDs: make([]string, 0),
		t:      t,
	}
}

// RecordJobID stores a job ID
func (v *JobIDValidator) RecordJobID(jobID string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.jobIDs = append(v.jobIDs, jobID)
	v.t.Logf("Captured job ID: %s", jobID)
}

// GetJobIDs returns all captured job IDs
func (v *JobIDValidator) GetJobIDs() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := make([]string, len(v.jobIDs))
	copy(result, v.jobIDs)
	return result
}

// GetJobIDsWithPrefix returns job IDs matching a prefix
func (v *JobIDValidator) GetJobIDsWithPrefix(prefix string) []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := make([]string, 0)
	for _, jobID := range v.jobIDs {
		if strings.HasPrefix(jobID, prefix) {
			result = append(result, jobID)
		}
	}
	return result
}

// Clear removes all job IDs
func (v *JobIDValidator) Clear() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.jobIDs = make([]string, 0)
}

// TestWorker is a custom worker for job ID validation tests
type TestWorker struct {
	consumer  *worker.Consumer
	validator *JobIDValidator
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewTestWorker creates a test worker that validates job IDs
func NewTestWorker(ctx context.Context, t *testing.T, q queue.Queuer, validator *JobIDValidator) *TestWorker {
	logger := testenv.NewLogger(t)
	logger.SetLevel(log.ErrorLevel)

	workerCtx, cancel := context.WithCancel(ctx)

	consumer := worker.NewConsumer(workerCtx, 3, q, logger, log.ErrorLevel)

	tw := &TestWorker{
		consumer:  consumer,
		validator: validator,
		ctx:       workerCtx,
		cancel:    cancel,
	}

	// Register handlers for all event processor types
	consumer.RegisterHandlers(convoy.CreateEventProcessor, tw.handleCreateEvent, nil)
	consumer.RegisterHandlers(convoy.CreateBroadcastEventProcessor, tw.handleCreateEvent, nil)
	consumer.RegisterHandlers(convoy.CreateDynamicEventProcessor, tw.handleCreateEvent, nil)

	return tw
}

// handleCreateEvent validates the job ID and records it
func (tw *TestWorker) handleCreateEvent(ctx context.Context, t *asynq.Task) error {
	// Get the job ID from the task
	jobID := t.ResultWriter().TaskID()

	// Record the job ID for validation
	tw.validator.RecordJobID(jobID)

	// Validate basic job ID format
	if !strings.Contains(jobID, ":") {
		return fmt.Errorf("invalid job ID format: %s (expected format with colons)", jobID)
	}

	// Success - job ID is valid
	return nil
}

// Start starts the test worker
func (tw *TestWorker) Start() {
	go func() {
		tw.consumer.Start()
	}()
}

// Stop stops the test worker
func (tw *TestWorker) Stop() {
	tw.cancel()
	tw.consumer.Stop()
}

// VerifyJobIDFormat verifies that a job ID has the expected format
func VerifyJobIDFormat(t *testing.T, jobID, expectedPrefix, projectID string) {
	t.Helper()

	expected := fmt.Sprintf("%s:%s:", expectedPrefix, projectID)
	if !strings.HasPrefix(jobID, expected) {
		t.Errorf("Job ID %s should start with %s", jobID, expected)
	}
}
