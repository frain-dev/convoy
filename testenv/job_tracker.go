package testenv

import (
	"sync"

	"github.com/olamilekan000/surge/surge/job"
)

// JobTracker captures job IDs during E2E tests for verification
type JobTracker struct {
	mu      sync.RWMutex
	jobIDs  []string
	enabled bool
}

// NewJobTracker creates a new job tracker for E2E tests
func NewJobTracker() *JobTracker {
	return &JobTracker{
		jobIDs:  make([]string, 0),
		enabled: true,
	}
}

// RecordJob records a job ID from a job envelope
func (jt *JobTracker) RecordJob(jobEnvelope *job.JobEnvelope) {
	if !jt.enabled {
		return
	}

	jt.mu.Lock()
	defer jt.mu.Unlock()

	// Record the job ID
	jt.jobIDs = append(jt.jobIDs, jobEnvelope.ID)
}

// GetJobIDs returns all recorded job IDs
func (jt *JobTracker) GetJobIDs() []string {
	jt.mu.RLock()
	defer jt.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]string, len(jt.jobIDs))
	copy(result, jt.jobIDs)
	return result
}

// GetJobIDsWithPrefix returns all job IDs that start with the given prefix
func (jt *JobTracker) GetJobIDsWithPrefix(prefix string) []string {
	jt.mu.RLock()
	defer jt.mu.RUnlock()

	result := make([]string, 0)
	for _, jobID := range jt.jobIDs {
		if len(jobID) >= len(prefix) && jobID[:len(prefix)] == prefix {
			result = append(result, jobID)
		}
	}
	return result
}

// Clear removes all recorded job IDs
func (jt *JobTracker) Clear() {
	jt.mu.Lock()
	defer jt.mu.Unlock()

	jt.jobIDs = make([]string, 0)
}

// Disable stops recording job IDs
func (jt *JobTracker) Disable() {
	jt.mu.Lock()
	defer jt.mu.Unlock()

	jt.enabled = false
}

// Enable starts recording job IDs
func (jt *JobTracker) Enable() {
	jt.mu.Lock()
	defer jt.mu.Unlock()

	jt.enabled = true
}
