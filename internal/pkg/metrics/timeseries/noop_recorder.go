package timeseries

import (
	"context"
	"time"
)

// NoOpMetricsRecorder is a no-op implementation of MetricsRecorder
// Used when metrics are disabled
type NoOpMetricsRecorder struct{}

// NewNoOpMetricsRecorder creates a new no-op metrics recorder
func NewNoOpMetricsRecorder() MetricsRecorder {
	return &NoOpMetricsRecorder{}
}

// RecordEventCreated does nothing
func (n *NoOpMetricsRecorder) RecordEventCreated(ctx context.Context, projectID, sourceID string) error {
	return nil
}

// RecordEventStatusChange does nothing
func (n *NoOpMetricsRecorder) RecordEventStatusChange(ctx context.Context, projectID, sourceID, oldStatus, newStatus string) error {
	return nil
}

// RecordEventDeliveryCreated does nothing
func (n *NoOpMetricsRecorder) RecordEventDeliveryCreated(ctx context.Context, ed *EventDeliveryMetrics) error {
	return nil
}

// RecordEventDeliveryStatusChange does nothing
func (n *NoOpMetricsRecorder) RecordEventDeliveryStatusChange(ctx context.Context, ed *EventDeliveryMetrics, oldStatus, newStatus string) error {
	return nil
}

// RecordDeliveryAttempt does nothing
func (n *NoOpMetricsRecorder) RecordDeliveryAttempt(ctx context.Context, attempt *DeliveryAttemptMetrics) error {
	return nil
}

// SetOldestPendingEvent does nothing
func (n *NoOpMetricsRecorder) SetOldestPendingEvent(ctx context.Context, projectID, sourceID string, timestamp time.Time) error {
	return nil
}

// ClearOldestPendingEvent does nothing
func (n *NoOpMetricsRecorder) ClearOldestPendingEvent(ctx context.Context, projectID, sourceID string) error {
	return nil
}

// SetOldestPendingDelivery does nothing
func (n *NoOpMetricsRecorder) SetOldestPendingDelivery(ctx context.Context, projectID, endpointID, sourceID string, timestamp time.Time) error {
	return nil
}

// ClearOldestPendingDelivery does nothing
func (n *NoOpMetricsRecorder) ClearOldestPendingDelivery(ctx context.Context, projectID, endpointID, sourceID string) error {
	return nil
}

// Close does nothing
func (n *NoOpMetricsRecorder) Close() error {
	return nil
}
