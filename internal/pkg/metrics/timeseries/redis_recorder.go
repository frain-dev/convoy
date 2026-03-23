package timeseries

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/redis/go-redis/v9"
)

// RedisMetricsRecorder implements MetricsRecorder using Redis Time Series
type RedisMetricsRecorder struct {
	client        redis.UniversalClient
	enabled       bool
	retentionMS   int64
	mu            sync.Mutex
	pipeline      redis.Pipeliner
	batchSize     int
	batchCount    int
	flushInterval time.Duration
	flushTimer    *time.Timer
	stopFlush     chan struct{}
	keyCache      map[string]bool
	keyCacheMu    sync.RWMutex
	autoFlushMu   sync.Mutex
	closeOnce     sync.Once
}

// NewRedisMetricsRecorder creates a new Redis Time Series metrics recorder
func NewRedisMetricsRecorder(client redis.UniversalClient, cfg config.TimeSeriesConfiguration) MetricsRecorder {
	if !cfg.Enabled || client == nil {
		return NewNoOpMetricsRecorder()
	}

	retentionMS := int64(cfg.RetentionSeconds * 1000)
	batchSize := cfg.PipelineBatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	flushInterval := time.Duration(cfg.FlushIntervalMs) * time.Millisecond
	if flushInterval <= 0 {
		flushInterval = 100 * time.Millisecond
	}

	recorder := &RedisMetricsRecorder{
		client:        client,
		enabled:       cfg.Enabled,
		retentionMS:   retentionMS,
		pipeline:      client.Pipeline(),
		batchSize:     batchSize,
		batchCount:    0,
		flushInterval: flushInterval,
		stopFlush:     make(chan struct{}),
		keyCache:      make(map[string]bool),
	}

	// Start auto-flush goroutine
	go recorder.autoFlush()

	return recorder
}

// autoFlush periodically flushes the pipeline
func (r *RedisMetricsRecorder) autoFlush() {
	ticker := time.NewTicker(r.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.autoFlushMu.Lock()
			if err := r.flush(context.Background()); err != nil {
				log.WithError(err).Error("failed to auto-flush metrics pipeline")
			}
			r.autoFlushMu.Unlock()
		case <-r.stopFlush:
			return
		}
	}
}

// ensureTimeSeriesExists ensures a time series exists with proper configuration
func (r *RedisMetricsRecorder) ensureTimeSeriesExists(ctx context.Context, key string, labels map[string]string) error {
	// Check cache first
	r.keyCacheMu.RLock()
	exists := r.keyCache[key]
	r.keyCacheMu.RUnlock()

	if exists {
		return nil
	}

	// Try to create the time series (idempotent operation)
	args := []interface{}{"TS.CREATE", key, "RETENTION", r.retentionMS}

	// Add labels
	if len(labels) > 0 {
		args = append(args, "LABELS")
		for k, v := range labels {
			args = append(args, k, v)
		}
	}

	if err := r.client.Do(ctx, args...).Err(); err != nil {
		// TSDB: key already exists - this is fine
		if err.Error() != "TSDB: key already exists" && err.Error() != "ERR TSDB: key already exists" {
			return fmt.Errorf("failed to create time series %s: %w", key, err)
		}
	}

	// Cache the key
	r.keyCacheMu.Lock()
	r.keyCache[key] = true
	r.keyCacheMu.Unlock()

	return nil
}

// incrBy increments a time series counter
func (r *RedisMetricsRecorder) incrBy(ctx context.Context, key string, value int64, labels map[string]string) error {
	if !r.enabled {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure time series exists
	if err := r.ensureTimeSeriesExists(ctx, key, labels); err != nil {
		log.WithError(err).Errorf("failed to ensure time series exists: %s", key)
		return err
	}

	// Use TS.INCRBY/TS.DECRBY command
	var cmd string
	if value >= 0 {
		cmd = "TS.INCRBY"
	} else {
		cmd = "TS.DECRBY"
		value = -value
	}

	r.pipeline.Do(ctx, cmd, key, value)
	r.batchCount++

	// Auto-flush if batch size reached
	if r.batchCount >= r.batchSize {
		return r.flush(ctx)
	}

	return nil
}

// add adds a sample to a time series
func (r *RedisMetricsRecorder) add(ctx context.Context, key string, timestamp int64, value float64, labels map[string]string) error {
	if !r.enabled {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Ensure time series exists
	if err := r.ensureTimeSeriesExists(ctx, key, labels); err != nil {
		log.WithError(err).Errorf("failed to ensure time series exists: %s", key)
		return err
	}

	// Use TS.ADD command
	r.pipeline.Do(ctx, "TS.ADD", key, timestamp, value)
	r.batchCount++

	// Auto-flush if batch size reached
	if r.batchCount >= r.batchSize {
		return r.flush(ctx)
	}

	return nil
}

// flush executes the pipeline
func (r *RedisMetricsRecorder) flush(ctx context.Context) error {
	if r.batchCount == 0 {
		return nil
	}

	_, err := r.pipeline.Exec(ctx)
	if err != nil {
		log.WithError(err).Error("failed to execute metrics pipeline")
		// Don't return error - we don't want metrics to block business logic
	}

	// Reset pipeline and counter
	r.pipeline = r.client.Pipeline()
	r.batchCount = 0

	return err
}

// RecordEventCreated records an event creation
func (r *RedisMetricsRecorder) RecordEventCreated(ctx context.Context, projectID, sourceID string) error {
	key := EventQueueKey(projectID, sourceID)
	labels := EventQueueLabels(projectID, sourceID)
	return r.incrBy(ctx, key, 1, labels)
}

// RecordEventStatusChange records an event status change
func (r *RedisMetricsRecorder) RecordEventStatusChange(ctx context.Context, projectID, sourceID, oldStatus, newStatus string) error {
	// For event status changes, we just update the counter
	// In this simple model, we track total events in queue
	// When an event completes (moves to success/failure), decrement
	if newStatus == "success" || newStatus == "failure" {
		key := EventQueueKey(projectID, sourceID)
		labels := EventQueueLabels(projectID, sourceID)
		return r.incrBy(ctx, key, -1, labels)
	}
	return nil
}

// RecordEventDeliveryCreated records an event delivery creation
func (r *RedisMetricsRecorder) RecordEventDeliveryCreated(ctx context.Context, ed *EventDeliveryMetrics) error {
	key := EventDeliveryQueueKey(ed.ProjectID, ed.EndpointID, ed.Status, ed.EventType, ed.SourceID, ed.OrgID)
	labels := EventDeliveryLabels(ed)
	return r.incrBy(ctx, key, 1, labels)
}

// RecordEventDeliveryStatusChange records an event delivery status change
func (r *RedisMetricsRecorder) RecordEventDeliveryStatusChange(ctx context.Context, ed *EventDeliveryMetrics, oldStatus, newStatus string) error {
	// Decrement old status counter
	if oldStatus != "" {
		oldKey := EventDeliveryQueueKey(ed.ProjectID, ed.EndpointID, oldStatus, ed.EventType, ed.SourceID, ed.OrgID)
		oldMetrics := *ed
		oldMetrics.Status = oldStatus
		oldLabels := EventDeliveryLabels(&oldMetrics)
		if err := r.incrBy(ctx, oldKey, -1, oldLabels); err != nil {
			log.WithError(err).Warnf("failed to decrement old status counter: %s", oldKey)
		}
	}

	// Increment new status counter
	newKey := EventDeliveryQueueKey(ed.ProjectID, ed.EndpointID, newStatus, ed.EventType, ed.SourceID, ed.OrgID)
	newMetrics := *ed
	newMetrics.Status = newStatus
	newLabels := EventDeliveryLabels(&newMetrics)
	return r.incrBy(ctx, newKey, 1, newLabels)
}

// RecordDeliveryAttempt records a delivery attempt
func (r *RedisMetricsRecorder) RecordDeliveryAttempt(ctx context.Context, attempt *DeliveryAttemptMetrics) error {
	key := DeliveryAttemptsKey(attempt.ProjectID, attempt.EndpointID, attempt.Status, strconv.Itoa(attempt.StatusCode))
	labels := DeliveryAttemptLabels(attempt)
	// Use current timestamp and value 1 for counter behavior
	return r.add(ctx, key, time.Now().UnixMilli(), 1.0, labels)
}

// SetOldestPendingEvent sets the timestamp of the oldest pending event
func (r *RedisMetricsRecorder) SetOldestPendingEvent(ctx context.Context, projectID, sourceID string, timestamp time.Time) error {
	if !r.enabled {
		return nil
	}

	key := EventBacklogKey(projectID, sourceID)
	return r.client.Set(ctx, key, timestamp.Unix(), 0).Err()
}

// ClearOldestPendingEvent clears the oldest pending event timestamp
func (r *RedisMetricsRecorder) ClearOldestPendingEvent(ctx context.Context, projectID, sourceID string) error {
	if !r.enabled {
		return nil
	}

	key := EventBacklogKey(projectID, sourceID)
	return r.client.Del(ctx, key).Err()
}

// SetOldestPendingDelivery sets the timestamp of the oldest pending delivery
func (r *RedisMetricsRecorder) SetOldestPendingDelivery(ctx context.Context, projectID, endpointID, sourceID string, timestamp time.Time) error {
	if !r.enabled {
		return nil
	}

	key := DeliveryBacklogKey(projectID, endpointID, sourceID)
	return r.client.Set(ctx, key, timestamp.Unix(), 0).Err()
}

// ClearOldestPendingDelivery clears the oldest pending delivery timestamp
func (r *RedisMetricsRecorder) ClearOldestPendingDelivery(ctx context.Context, projectID, endpointID, sourceID string) error {
	if !r.enabled {
		return nil
	}

	key := DeliveryBacklogKey(projectID, endpointID, sourceID)
	return r.client.Del(ctx, key).Err()
}

// Close flushes any pending metrics and stops the auto-flush goroutine
func (r *RedisMetricsRecorder) Close() error {
	var err error
	r.closeOnce.Do(func() {
		// Stop auto-flush goroutine
		close(r.stopFlush)

		// Final flush
		r.autoFlushMu.Lock()
		r.mu.Lock()
		err = r.flush(context.Background())
		r.mu.Unlock()
		r.autoFlushMu.Unlock()
	})
	return err
}
