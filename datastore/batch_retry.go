package datastore

import (
	"errors"
	"gopkg.in/guregu/null.v4"
	"time"
)

var (
	ErrBatchRetryNotFound = errors.New("batch retry not found")
)

type BatchRetryStatus string

const (
	BatchRetryStatusPending    BatchRetryStatus = "pending"
	BatchRetryStatusProcessing BatchRetryStatus = "processing"
	BatchRetryStatusCompleted  BatchRetryStatus = "completed"
	BatchRetryStatusFailed     BatchRetryStatus = "failed"
)

type BatchRetry struct {
	ID              string           `json:"uid" db:"id"`
	ProjectID       string           `json:"project_id" db:"project_id"`
	Status          BatchRetryStatus `json:"status" db:"status"`
	TotalEvents     int              `json:"total_events" db:"total_events"`
	ProcessedEvents int              `json:"processed_events" db:"processed_events"`
	FailedEvents    int              `json:"failed_events" db:"failed_events"`
	Error           string           `json:"error" db:"error"`
	Filter          *Filter          `json:"filter" db:"filter"`
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at" db:"updated_at"`
	CompletedAt     null.Time        `json:"completed_at" db:"completed_at"`
}
