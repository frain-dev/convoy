package timeseries

import (
	"context"
	"time"
)

// MetricsRecorder defines the interface for recording metrics in real-time
type MetricsRecorder interface {
	// Event lifecycle
	RecordEventCreated(ctx context.Context, projectID, sourceID string) error
	RecordEventStatusChange(ctx context.Context, projectID, sourceID, oldStatus, newStatus string) error

	// Event Delivery lifecycle
	RecordEventDeliveryCreated(ctx context.Context, ed *EventDeliveryMetrics) error
	RecordEventDeliveryStatusChange(ctx context.Context, ed *EventDeliveryMetrics, oldStatus, newStatus string) error

	// Delivery attempts
	RecordDeliveryAttempt(ctx context.Context, attempt *DeliveryAttemptMetrics) error

	// Backlog tracking
	SetOldestPendingEvent(ctx context.Context, projectID, sourceID string, timestamp time.Time) error
	ClearOldestPendingEvent(ctx context.Context, projectID, sourceID string) error
	SetOldestPendingDelivery(ctx context.Context, projectID, endpointID, sourceID string, timestamp time.Time) error
	ClearOldestPendingDelivery(ctx context.Context, projectID, endpointID, sourceID string) error

	// Lifecycle
	Close() error
}

// EventDeliveryMetrics contains metrics data for event deliveries
// Slim structs to avoid pulling in full datastore models
type EventDeliveryMetrics struct {
	ProjectID   string
	ProjectName string
	EndpointID  string
	EventID     string
	EventType   string
	SourceID    string
	OrgID       string
	OrgName     string
	Status      string
}

// DeliveryAttemptMetrics contains metrics data for delivery attempts
type DeliveryAttemptMetrics struct {
	ProjectID  string
	EndpointID string
	EventType  string
	Status     string
	StatusCode int
}
