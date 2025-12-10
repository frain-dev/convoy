package batch_tracker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"
)

const (
	batchKeyPrefix = "batch:retry:"
	batchTTL       = 24 * time.Hour // Keep batch data for 24 hours
)

type BatchStatus string

const (
	BatchStatusRunning   BatchStatus = "running"
	BatchStatusCompleted BatchStatus = "completed"
	BatchStatusFailed    BatchStatus = "failed"
)

type BatchProgress struct {
	BatchID        string      `json:"batch_id"`
	Status         BatchStatus `json:"status"`
	TotalCount     int64       `json:"total_count"`
	ProcessedCount int64       `json:"processed_count"`
	FailedCount    int64       `json:"failed_count"`
	StartTime      time.Time   `json:"start_time"`
	EndTime        *time.Time  `json:"end_time,omitempty"`
	Error          string      `json:"error,omitempty"`
	StatusFilter   string      `json:"status_filter,omitempty"` // The event delivery status that was filtered (e.g., "Retry", "Scheduled")
	TimePeriod     string      `json:"time_period,omitempty"`   // The time period selected (e.g., "1h", "5h")
	EventID        string      `json:"event_id,omitempty"`      // Optional event ID filter
}

type BatchTracker struct {
	redis redis.UniversalClient
}

func NewBatchTracker(redis redis.UniversalClient) *BatchTracker {
	return &BatchTracker{redis: redis}
}

// GenerateBatchID generates a unique batch ID
func (bt *BatchTracker) GenerateBatchID() string {
	return ulid.Make().String()
}

// CreateBatch initializes a new batch in Redis
func (bt *BatchTracker) CreateBatch(ctx context.Context, batchID string, totalCount int64, statusFilter, timePeriod, eventID string) error {
	progress := &BatchProgress{
		BatchID:        batchID,
		Status:         BatchStatusRunning,
		TotalCount:     totalCount,
		ProcessedCount: 0,
		FailedCount:    0,
		StartTime:      time.Now(),
		StatusFilter:   statusFilter,
		TimePeriod:     timePeriod,
		EventID:        eventID,
	}

	// Initialize counters (they'll be incremented as we process)
	baseKey := bt.getBatchKey(batchID)
	if err := bt.redis.Set(ctx, baseKey+":total", 0, batchTTL).Err(); err != nil {
		return err
	}
	if err := bt.redis.Set(ctx, baseKey+":processed", 0, batchTTL).Err(); err != nil {
		return err
	}
	if err := bt.redis.Set(ctx, baseKey+":failed", 0, batchTTL).Err(); err != nil {
		return err
	}

	return bt.updateBatch(ctx, batchID, progress)
}

// UpdateProgress updates the processed count for a batch
func (bt *BatchTracker) UpdateProgress(ctx context.Context, batchID string, processedCount, failedCount int64) error {
	progress, err := bt.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}

	progress.ProcessedCount = processedCount
	progress.FailedCount = failedCount

	return bt.updateBatch(ctx, batchID, progress)
}

// CompleteBatch marks a batch as completed
func (bt *BatchTracker) CompleteBatch(ctx context.Context, batchID string) error {
	progress, err := bt.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}

	now := time.Now()
	progress.Status = BatchStatusCompleted
	progress.EndTime = &now

	return bt.updateBatch(ctx, batchID, progress)
}

// FailBatch marks a batch as failed with an error message
func (bt *BatchTracker) FailBatch(ctx context.Context, batchID, errMsg string) error {
	progress, err := bt.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}

	now := time.Now()
	progress.Status = BatchStatusFailed
	progress.EndTime = &now
	progress.Error = errMsg

	return bt.updateBatch(ctx, batchID, progress)
}

// GetBatch retrieves batch progress from Redis
func (bt *BatchTracker) GetBatch(ctx context.Context, batchID string) (*BatchProgress, error) {
	key := bt.getBatchKey(batchID)

	data, err := bt.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("batch not found: %s", batchID)
		}
		return nil, err
	}

	var progress BatchProgress
	if err := json.Unmarshal([]byte(data), &progress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal batch progress: %w", err)
	}

	return &progress, nil
}

// IncrementProcessed atomically increments the processed count using Redis INCR
func (bt *BatchTracker) IncrementProcessed(ctx context.Context, batchID string, count int64) error {
	key := bt.getBatchKey(batchID) + ":processed"
	return bt.redis.IncrBy(ctx, key, count).Err()
}

// IncrementFailed atomically increments the failed count using Redis INCR
func (bt *BatchTracker) IncrementFailed(ctx context.Context, batchID string, count int64) error {
	key := bt.getBatchKey(batchID) + ":failed"
	return bt.redis.IncrBy(ctx, key, count).Err()
}

// IncrementTotal atomically increments the total count using Redis INCR
func (bt *BatchTracker) IncrementTotal(ctx context.Context, batchID string, count int64) error {
	key := bt.getBatchKey(batchID) + ":total"
	return bt.redis.IncrBy(ctx, key, count).Err()
}

// SyncCounters updates the main batch progress with current counter values
func (bt *BatchTracker) SyncCounters(ctx context.Context, batchID string) error {
	progress, err := bt.GetBatch(ctx, batchID)
	if err != nil {
		return err
	}

	baseKey := bt.getBatchKey(batchID)

	// Get current counter values
	processed, err := bt.redis.Get(ctx, baseKey+":processed").Int64()
	if err != nil && err != redis.Nil {
		return err
	}
	if err == redis.Nil {
		processed = 0
	}

	failed, err := bt.redis.Get(ctx, baseKey+":failed").Int64()
	if err != nil && err != redis.Nil {
		return err
	}
	if err == redis.Nil {
		failed = 0
	}

	// Get total from counter if it exists, otherwise use the progress total
	total, err := bt.redis.Get(ctx, baseKey+":total").Int64()
	if err != nil && err != redis.Nil {
		return err
	}
	if err == redis.Nil {
		total = progress.TotalCount // Use existing total if counter doesn't exist
	}

	// Update progress with synced counters
	progress.ProcessedCount = processed
	progress.FailedCount = failed
	progress.TotalCount = total

	return bt.updateBatch(ctx, batchID, progress)
}

// updateBatch stores batch progress in Redis
func (bt *BatchTracker) updateBatch(ctx context.Context, batchID string, progress *BatchProgress) error {
	key := bt.getBatchKey(batchID)

	data, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("failed to marshal batch progress: %w", err)
	}

	return bt.redis.Set(ctx, key, data, batchTTL).Err()
}

// ListBatches retrieves all batches from Redis
func (bt *BatchTracker) ListBatches(ctx context.Context) ([]*BatchProgress, error) {
	pattern := bt.getBatchKey("*")

	keys, err := bt.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list batch keys: %w", err)
	}

	batches := make([]*BatchProgress, 0, len(keys))
	for _, key := range keys {
		// Extract batch ID from key (remove prefix)
		batchID := key[len(batchKeyPrefix):]

		progress, err := bt.GetBatch(ctx, batchID)
		if err != nil {
			// Skip batches that can't be retrieved (might be in inconsistent state)
			continue
		}

		batches = append(batches, progress)
	}

	return batches, nil
}

// DeleteBatch removes a batch and all its counters from Redis
func (bt *BatchTracker) DeleteBatch(ctx context.Context, batchID string) error {
	baseKey := bt.getBatchKey(batchID)

	// Delete all keys related to this batch
	keys := []string{
		baseKey,                // Main batch data
		baseKey + ":total",     // Total counter
		baseKey + ":processed", // Processed counter
		baseKey + ":failed",    // Failed counter
	}

	for _, key := range keys {
		if err := bt.redis.Del(ctx, key).Err(); err != nil {
			// Log error but continue deleting other keys
			// Return error only if it's not a "key doesn't exist" error
			if err != redis.Nil {
				return fmt.Errorf("failed to delete batch key %s: %w", key, err)
			}
		}
	}

	return nil
}

func (bt *BatchTracker) getBatchKey(batchID string) string {
	return fmt.Sprintf("%s%s", batchKeyPrefix, batchID)
}
