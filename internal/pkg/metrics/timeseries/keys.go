package timeseries

import (
	"fmt"
	"strings"
)

const (
	// Key prefixes
	metricsPrefix = "convoy:metrics"
	backlogPrefix = "convoy:backlog"

	// Metric names
	eventQueueTotalMetric         = "event_queue_total"
	eventBacklogMetric            = "event_backlog_ts"
	eventDeliveryQueueTotalMetric = "event_delivery_total"
	deliveryBacklogMetric         = "delivery_backlog_ts"
	deliveryAttemptsMetric        = "delivery_attempts"
)

// sanitize removes or replaces characters that could cause issues in Redis keys
func sanitize(s string) string {
	if s == "" {
		return "*"
	}
	// Replace colons with underscores as colons are used as key separators
	s = strings.ReplaceAll(s, ":", "_")
	return s
}

// EventQueueKey generates a key for event queue total metrics
func EventQueueKey(projectID, sourceID string) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		metricsPrefix,
		eventQueueTotalMetric,
		sanitize(projectID),
		sanitize(sourceID),
	)
}

// EventBacklogKey generates a key for event backlog timestamp tracking
func EventBacklogKey(projectID, sourceID string) string {
	return fmt.Sprintf("%s:%s:%s:%s",
		backlogPrefix,
		eventBacklogMetric,
		sanitize(projectID),
		sanitize(sourceID),
	)
}

// EventDeliveryQueueKey generates a key for event delivery queue metrics
func EventDeliveryQueueKey(projectID, endpointID, status, eventType, sourceID, orgID string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s",
		metricsPrefix,
		eventDeliveryQueueTotalMetric,
		sanitize(projectID),
		sanitize(endpointID),
		sanitize(status),
		sanitize(eventType),
		sanitize(sourceID),
		sanitize(orgID),
	)
}

// DeliveryBacklogKey generates a key for delivery backlog timestamp tracking
func DeliveryBacklogKey(projectID, endpointID, sourceID string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s",
		backlogPrefix,
		deliveryBacklogMetric,
		sanitize(projectID),
		sanitize(endpointID),
		sanitize(sourceID),
	)
}

// DeliveryAttemptsKey generates a key for delivery attempts counter
func DeliveryAttemptsKey(projectID, endpointID, status, statusCode string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		metricsPrefix,
		deliveryAttemptsMetric,
		sanitize(projectID),
		sanitize(endpointID),
		sanitize(status),
		sanitize(statusCode),
	)
}

// EventQueueLabels generates labels for event queue metrics
func EventQueueLabels(projectID, sourceID string) map[string]string {
	return map[string]string{
		"metric":  eventQueueTotalMetric,
		"project": sanitize(projectID),
		"source":  sanitize(sourceID),
		"status":  "success",
	}
}

// EventDeliveryLabels generates labels for event delivery metrics
func EventDeliveryLabels(ed *EventDeliveryMetrics) map[string]string {
	return map[string]string{
		"metric":       eventDeliveryQueueTotalMetric,
		"project":      sanitize(ed.ProjectID),
		"project_name": sanitize(ed.ProjectName),
		"endpoint":     sanitize(ed.EndpointID),
		"status":       sanitize(ed.Status),
		"event_type":   sanitize(ed.EventType),
		"source":       sanitize(ed.SourceID),
		"org":          sanitize(ed.OrgID),
		"org_name":     sanitize(ed.OrgName),
	}
}

// DeliveryAttemptLabels generates labels for delivery attempt metrics
func DeliveryAttemptLabels(attempt *DeliveryAttemptMetrics) map[string]string {
	return map[string]string{
		"metric":      deliveryAttemptsMetric,
		"project":     sanitize(attempt.ProjectID),
		"endpoint":    sanitize(attempt.EndpointID),
		"status":      sanitize(attempt.Status),
		"status_code": fmt.Sprintf("%d", attempt.StatusCode),
		"event_type":  sanitize(attempt.EventType),
	}
}
