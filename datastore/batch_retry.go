package datastore

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gopkg.in/guregu/null.v4"
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
	Filter          RetryFilter      `json:"filter" db:"filter"`
	CreatedAt       time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at" db:"updated_at"`
	CompletedAt     null.Time        `json:"completed_at" db:"completed_at"`
}

type RetryFilter map[string]any

func FromFilterStruct(data Filter) RetryFilter {
	return RetryFilter{
		"Query":          data.Query,
		"OwnerID":        data.OwnerID,
		"Project":        data.Project,
		"ProjectID":      data.ProjectID,
		"EndpointID":     data.EndpointID,
		"EndpointIDs":    data.EndpointIDs,
		"SubscriptionID": data.SubscriptionID,
		"EventID":        data.EventID,
		"EventType":      data.EventType,
		"SourceID":       data.SourceID,
		"SourceIDs":      data.SourceIDs,
		"Pageable": map[string]any{
			"per_page":         data.Pageable.PerPage,
			"direction":        data.Pageable.Direction,
			"sort":             data.Pageable.Sort,
			"prev_page_cursor": data.Pageable.PrevCursor,
			"next_page_cursor": data.Pageable.NextCursor,
		},
		"IdempotencyKey": data.IdempotencyKey,
		"Status":         data.Status,
		"SearchParams": map[string]any{
			"created_at_start": data.SearchParams.CreatedAtStart,
			"created_at_end":   data.SearchParams.CreatedAtEnd,
		},
	}
}

func (f *RetryFilter) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	return json.Unmarshal(bytes, f)
}

func (b *BatchRetry) GetFilter() (*Filter, error) {
	bytes, err := json.Marshal(b.Filter)
	if err != nil {
		return nil, err
	}

	filter := Filter{}
	err = json.Unmarshal(bytes, &filter)
	if err != nil {
		return nil, err
	}

	return &filter, err
}
