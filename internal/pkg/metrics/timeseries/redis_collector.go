package timeseries

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// RedisTimeSeriesCollector implements prometheus.Collector for Redis Time Series
type RedisTimeSeriesCollector struct {
	client     redis.UniversalClient
	enabled    bool
	sampleTime time.Duration

	// Metric descriptors
	eventQueueTotalDesc        *prometheus.Desc
	eventBacklogSecondsDesc    *prometheus.Desc
	deliveryQueueTotalDesc     *prometheus.Desc
	deliveryBacklogSecondsDesc *prometheus.Desc
	deliveryAttemptsTotalDesc  *prometheus.Desc

	// Caching
	cachedMetrics *CachedMetrics
	lastRun       time.Time
	mu            sync.RWMutex
}

// CachedMetrics stores cached metric values
type CachedMetrics struct {
	EventQueueMetrics      []prometheus.Metric
	EventBacklogMetrics    []prometheus.Metric
	DeliveryQueueMetrics   []prometheus.Metric
	DeliveryBacklogMetrics []prometheus.Metric
	DeliveryAttemptMetrics []prometheus.Metric
}

// NewRedisTimeSeriesCollector creates a new Redis Time Series collector
func NewRedisTimeSeriesCollector(client redis.UniversalClient, cfg config.TimeSeriesConfiguration) *RedisTimeSeriesCollector {
	sampleTime := time.Duration(cfg.SampleTime) * time.Second
	if sampleTime <= 0 {
		sampleTime = 5 * time.Second
	}

	return &RedisTimeSeriesCollector{
		client:     client,
		enabled:    cfg.Enabled,
		sampleTime: sampleTime,

		eventQueueTotalDesc: prometheus.NewDesc(
			"convoy_event_queue_total",
			"Total number of events in queue",
			[]string{"project", "source", "status"},
			nil,
		),

		eventBacklogSecondsDesc: prometheus.NewDesc(
			"convoy_event_queue_backlog_seconds",
			"Age of oldest pending event in seconds",
			[]string{"project", "source"},
			nil,
		),

		deliveryQueueTotalDesc: prometheus.NewDesc(
			"convoy_event_delivery_queue_total",
			"Total number of event deliveries in queue",
			[]string{"project", "project_name", "endpoint", "status", "event_type", "source", "org", "org_name"},
			nil,
		),

		deliveryBacklogSecondsDesc: prometheus.NewDesc(
			"convoy_event_delivery_queue_backlog_seconds",
			"Age of oldest pending delivery in seconds",
			[]string{"project", "endpoint", "source"},
			nil,
		),

		deliveryAttemptsTotalDesc: prometheus.NewDesc(
			"convoy_event_delivery_attempts_total",
			"Total number of delivery attempts",
			[]string{"project", "endpoint", "status", "status_code", "event_type"},
			nil,
		),
	}
}

// Describe implements prometheus.Collector
func (rc *RedisTimeSeriesCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rc.eventQueueTotalDesc
	ch <- rc.eventBacklogSecondsDesc
	ch <- rc.deliveryQueueTotalDesc
	ch <- rc.deliveryBacklogSecondsDesc
	ch <- rc.deliveryAttemptsTotalDesc
}

// Collect implements prometheus.Collector
func (rc *RedisTimeSeriesCollector) Collect(ch chan<- prometheus.Metric) {
	if !rc.enabled {
		return
	}

	// Check cache validity
	rc.mu.RLock()
	cacheValid := rc.cachedMetrics != nil && time.Since(rc.lastRun) < rc.sampleTime
	if cacheValid {
		rc.emitCachedMetrics(ch)
		rc.mu.RUnlock()
		return
	}
	rc.mu.RUnlock()

	// Collect fresh metrics
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cached := &CachedMetrics{}

	// Collect event queue metrics
	cached.EventQueueMetrics = rc.collectEventQueueMetrics(ctx, ch)

	// Collect event backlog metrics
	cached.EventBacklogMetrics = rc.collectEventBacklogMetrics(ctx, ch)

	// Collect delivery queue metrics
	cached.DeliveryQueueMetrics = rc.collectDeliveryQueueMetrics(ctx, ch)

	// Collect delivery backlog metrics
	cached.DeliveryBacklogMetrics = rc.collectDeliveryBacklogMetrics(ctx, ch)

	// Collect delivery attempt metrics
	cached.DeliveryAttemptMetrics = rc.collectDeliveryAttemptMetrics(ctx, ch)

	// Update cache
	rc.mu.Lock()
	rc.cachedMetrics = cached
	rc.lastRun = time.Now()
	rc.mu.Unlock()
}

// emitCachedMetrics sends cached metrics to the channel
func (rc *RedisTimeSeriesCollector) emitCachedMetrics(ch chan<- prometheus.Metric) {
	if rc.cachedMetrics == nil {
		return
	}

	for _, m := range rc.cachedMetrics.EventQueueMetrics {
		ch <- m
	}
	for _, m := range rc.cachedMetrics.EventBacklogMetrics {
		ch <- m
	}
	for _, m := range rc.cachedMetrics.DeliveryQueueMetrics {
		ch <- m
	}
	for _, m := range rc.cachedMetrics.DeliveryBacklogMetrics {
		ch <- m
	}
	for _, m := range rc.cachedMetrics.DeliveryAttemptMetrics {
		ch <- m
	}
}

// collectEventQueueMetrics collects event queue total metrics
func (rc *RedisTimeSeriesCollector) collectEventQueueMetrics(ctx context.Context, ch chan<- prometheus.Metric) []prometheus.Metric {
	var metrics []prometheus.Metric

	// Query all event queue metrics using label filter
	cmd := rc.client.Do(ctx, "TS.MGET", "FILTER", fmt.Sprintf("metric=%s", eventQueueTotalMetric), "WITHLABELS")
	result, err := cmd.Result()
	if err != nil {
		log.WithError(err).Warn("failed to query event queue metrics")
		return metrics
	}

	// Parse results
	results, ok := result.([]interface{})
	if !ok {
		return metrics
	}

	for _, r := range results {
		metric := rc.parseTimeSeriesResult(r, rc.eventQueueTotalDesc)
		if metric != nil {
			metrics = append(metrics, metric)
			ch <- metric
		}
	}

	return metrics
}

// collectEventBacklogMetrics collects event backlog metrics
func (rc *RedisTimeSeriesCollector) collectEventBacklogMetrics(ctx context.Context, ch chan<- prometheus.Metric) []prometheus.Metric {
	var metrics []prometheus.Metric

	// Get all backlog timestamp keys
	pattern := fmt.Sprintf("%s:%s:*", backlogPrefix, eventBacklogMetric)
	keys, err := rc.client.Keys(ctx, pattern).Result()
	if err != nil {
		log.WithError(err).Warn("failed to query event backlog keys")
		return metrics
	}

	if len(keys) == 0 {
		return metrics
	}

	// Get all timestamps
	timestamps, err := rc.client.MGet(ctx, keys...).Result()
	if err != nil {
		log.WithError(err).Warn("failed to query event backlog timestamps")
		return metrics
	}

	now := time.Now().Unix()
	for i, key := range keys {
		if timestamps[i] == nil {
			continue
		}

		tsStr, ok := timestamps[i].(string)
		if !ok {
			continue
		}

		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}

		// Calculate backlog age in seconds
		age := float64(now - ts)
		if age < 0 {
			age = 0
		}

		// Extract project and source from key
		// Key format: convoy:backlog:event_backlog_ts:project:source
		labels := extractLabelsFromBacklogKey(key, "event")
		if len(labels) >= 2 {
			metric := prometheus.MustNewConstMetric(
				rc.eventBacklogSecondsDesc,
				prometheus.GaugeValue,
				age,
				labels[0], labels[1],
			)
			metrics = append(metrics, metric)
			ch <- metric
		}
	}

	return metrics
}

// collectDeliveryQueueMetrics collects delivery queue metrics
func (rc *RedisTimeSeriesCollector) collectDeliveryQueueMetrics(ctx context.Context, ch chan<- prometheus.Metric) []prometheus.Metric {
	var metrics []prometheus.Metric

	// Query all delivery queue metrics using label filter
	cmd := rc.client.Do(ctx, "TS.MGET", "FILTER", fmt.Sprintf("metric=%s", eventDeliveryQueueTotalMetric), "WITHLABELS")
	result, err := cmd.Result()
	if err != nil {
		log.WithError(err).Warn("failed to query delivery queue metrics")
		return metrics
	}

	// Parse results
	results, ok := result.([]interface{})
	if !ok {
		return metrics
	}

	for _, r := range results {
		metric := rc.parseTimeSeriesResult(r, rc.deliveryQueueTotalDesc)
		if metric != nil {
			metrics = append(metrics, metric)
			ch <- metric
		}
	}

	return metrics
}

// collectDeliveryBacklogMetrics collects delivery backlog metrics
func (rc *RedisTimeSeriesCollector) collectDeliveryBacklogMetrics(ctx context.Context, ch chan<- prometheus.Metric) []prometheus.Metric {
	var metrics []prometheus.Metric

	// Get all delivery backlog timestamp keys
	pattern := fmt.Sprintf("%s:%s:*", backlogPrefix, deliveryBacklogMetric)
	keys, err := rc.client.Keys(ctx, pattern).Result()
	if err != nil {
		log.WithError(err).Warn("failed to query delivery backlog keys")
		return metrics
	}

	if len(keys) == 0 {
		return metrics
	}

	// Get all timestamps
	timestamps, err := rc.client.MGet(ctx, keys...).Result()
	if err != nil {
		log.WithError(err).Warn("failed to query delivery backlog timestamps")
		return metrics
	}

	now := time.Now().Unix()
	for i, key := range keys {
		if timestamps[i] == nil {
			continue
		}

		tsStr, ok := timestamps[i].(string)
		if !ok {
			continue
		}

		ts, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			continue
		}

		// Calculate backlog age in seconds
		age := float64(now - ts)
		if age < 0 {
			age = 0
		}

		// Extract project, endpoint, and source from key
		// Key format: convoy:backlog:delivery_backlog_ts:project:endpoint:source
		labels := extractLabelsFromBacklogKey(key, "delivery")
		if len(labels) >= 3 {
			metric := prometheus.MustNewConstMetric(
				rc.deliveryBacklogSecondsDesc,
				prometheus.GaugeValue,
				age,
				labels[0], labels[1], labels[2],
			)
			metrics = append(metrics, metric)
			ch <- metric
		}
	}

	return metrics
}

// collectDeliveryAttemptMetrics collects delivery attempt metrics
func (rc *RedisTimeSeriesCollector) collectDeliveryAttemptMetrics(ctx context.Context, ch chan<- prometheus.Metric) []prometheus.Metric {
	var metrics []prometheus.Metric

	// Query all delivery attempt metrics using label filter
	cmd := rc.client.Do(ctx, "TS.MGET", "FILTER", fmt.Sprintf("metric=%s", deliveryAttemptsMetric), "WITHLABELS")
	result, err := cmd.Result()
	if err != nil {
		log.WithError(err).Warn("failed to query delivery attempt metrics")
		return metrics
	}

	// Parse results
	results, ok := result.([]interface{})
	if !ok {
		return metrics
	}

	for _, r := range results {
		metric := rc.parseTimeSeriesResult(r, rc.deliveryAttemptsTotalDesc)
		if metric != nil {
			metrics = append(metrics, metric)
			ch <- metric
		}
	}

	return metrics
}

// parseTimeSeriesResult parses a TS.MGET result and creates a Prometheus metric
func (rc *RedisTimeSeriesCollector) parseTimeSeriesResult(result interface{}, desc *prometheus.Desc) prometheus.Metric {
	// TS.MGET returns: [key, labels, [timestamp, value]]
	arr, ok := result.([]interface{})
	if !ok || len(arr) < 3 {
		return nil
	}

	// Extract labels (array of [label, value] pairs)
	labelsArr, ok := arr[1].([]interface{})
	if !ok {
		return nil
	}

	labels := make(map[string]string)
	for i := 0; i < len(labelsArr); i += 2 {
		if i+1 >= len(labelsArr) {
			break
		}
		labelKey, ok1 := labelsArr[i].(string)
		labelValue, ok2 := labelsArr[i+1].(string)
		if ok1 && ok2 {
			labels[labelKey] = labelValue
		}
	}

	// Extract value
	valueArr, ok := arr[2].([]interface{})
	if !ok || len(valueArr) < 2 {
		return nil
	}

	valueStr, ok := valueArr[1].(string)
	if !ok {
		return nil
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return nil
	}

	// Create metric based on descriptor
	var labelValues []string
	switch desc {
	case rc.eventQueueTotalDesc:
		labelValues = []string{
			labels["project"],
			labels["source"],
			labels["status"],
		}
	case rc.deliveryQueueTotalDesc:
		labelValues = []string{
			labels["project"],
			labels["project_name"],
			labels["endpoint"],
			labels["status"],
			labels["event_type"],
			labels["source"],
			labels["org"],
			labels["org_name"],
		}
	case rc.deliveryAttemptsTotalDesc:
		labelValues = []string{
			labels["project"],
			labels["endpoint"],
			labels["status"],
			labels["status_code"],
			labels["event_type"],
		}
	default:
		return nil
	}

	return prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value, labelValues...)
}

// extractLabelsFromBacklogKey extracts labels from a backlog key
func extractLabelsFromBacklogKey(key, keyType string) []string {
	// Key format: convoy:backlog:{metric_name}:{labels...}
	// For event: convoy:backlog:event_backlog_ts:project:source
	// For delivery: convoy:backlog:delivery_backlog_ts:project:endpoint:source

	parts := splitKey(key)
	if len(parts) < 4 {
		return nil
	}

	// Skip "convoy:backlog:{metric_name}" prefix
	if keyType == "event" && len(parts) >= 5 {
		return parts[3:5] // project, source
	} else if keyType == "delivery" && len(parts) >= 6 {
		return parts[3:6] // project, endpoint, source
	}

	return nil
}

// splitKey splits a Redis key by colons
func splitKey(key string) []string {
	var parts []string
	current := ""
	for _, ch := range key {
		if ch == ':' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
