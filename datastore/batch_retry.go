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
	filter := RetryFilter{
		"ProjectID":      data.ProjectID,
		"EndpointIDs":    data.EndpointIDs,
		"EventID":        data.EventID,
		"EventType":      data.EventType,
		"SourceID":       data.SourceID,
		"SourceIDs":      data.SourceIDs,
		"IdempotencyKey": data.IdempotencyKey,
		"Status":         data.Status,
	}

	// Only add non-nil string fields
	if data.Query != "" {
		filter["Query"] = data.Query
	}
	if data.OwnerID != "" {
		filter["OwnerID"] = data.OwnerID
	}
	if data.SubscriptionID != "" {
		filter["SubscriptionID"] = data.SubscriptionID
	}

	// Only add Project if it's not nil
	if data.Project != nil {
		// Skip Project field as it can cause marshaling issues
		// filter["Project"] = data.Project
	}

	// Only add EndpointID if it's not empty
	if data.EndpointID != "" {
		filter["EndpointID"] = data.EndpointID
	}

	// Only add Pageable if it has valid values
	if data.Pageable.PerPage > 0 || data.Pageable.Direction != "" || data.Pageable.Sort != "" || data.Pageable.PrevCursor != "" || data.Pageable.NextCursor != "" {
		filter["Pageable"] = map[string]any{
			"per_page":         data.Pageable.PerPage,
			"direction":        data.Pageable.Direction,
			"sort":             data.Pageable.Sort,
			"prev_page_cursor": data.Pageable.PrevCursor,
			"next_page_cursor": data.Pageable.NextCursor,
		}
	}

	// Only add SearchParams if it has valid values
	if data.SearchParams.CreatedAtStart != 0 || data.SearchParams.CreatedAtEnd != 0 {
		filter["SearchParams"] = map[string]any{
			"created_at_start": data.SearchParams.CreatedAtStart,
			"created_at_end":   data.SearchParams.CreatedAtEnd,
		}
	}

	return filter
}

func (f *RetryFilter) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	return json.Unmarshal(bytes, f)
}

func (b *BatchRetry) GetFilter() (*Filter, error) {
	if b.Filter == nil {
		return nil, fmt.Errorf("filter is nil")
	}

	bytes, err := json.Marshal(b.Filter)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal filter: %w", err)
	}

	filter := Filter{}
	err = json.Unmarshal(bytes, &filter)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal filter: %w", err)
	}

	return &filter, nil
}
