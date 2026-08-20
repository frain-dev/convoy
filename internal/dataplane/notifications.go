package dataplane

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/datastore"
	notification "github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

const breakerFailureMsg = "Circuit breaker threshold exceeded"

// EnqueueCircuitBreakerNotifications tells the endpoint's own channels (support
// email, Slack webhook and Teams webhook) that the breaker disabled it, and sends
// the organisation owner a separate email. ownerEmail may be empty when it could
// not be resolved; missing channels are skipped.
//
// Returns true when at least one notification job was written to the queue.
// Per-channel failures are logged rather than returned.
func EnqueueCircuitBreakerNotifications(ctx context.Context, q queue.Queuer, lo log.Logger, licenser license.Licenser, project *datastore.Project, endpoint *datastore.Endpoint, ownerEmail string, failureRate float64) bool {
	var enqueued bool

	if endpoint != nil && licenser.AdvancedEndpointMgmt() {
		enqueued = notification.DispatchEndpointAlert(ctx, q, lo, notification.EndpointAlert{
			EmailRecipient:  endpoint.SupportEmail,
			SlackWebhookURL: endpoint.SlackWebhookURL,
			TeamsWebhookURL: endpoint.TeamsWebhookURL,
			EmailSubject:    "Endpoint Disabled - Circuit Breaker Triggered",
			EmailParams: map[string]string{
				"name":            endpoint.Name,
				"logo_url":        project.LogoURL,
				"target_url":      endpoint.Url,
				"failure_msg":     breakerFailureMsg,
				"response_body":   "",
				"failure_rate":    fmt.Sprintf("%.2f%%", failureRate),
				"status_code":     "0",
				"endpoint_status": string(datastore.InactiveEndpointStatus),
			},
			AlertText: fmt.Sprintf("endpoint url (%s) has been disabled, reason for failure is %q with a failure rate of %.2f%%, endpoint status is now %s",
				endpoint.Url, breakerFailureMsg, failureRate, datastore.InactiveEndpointStatus),
		}) || enqueued
	}

	if ownerEmail == "" {
		return enqueued
	}

	nameParam := project.Name
	targetURL := ""
	failureRateStr := ""
	if endpoint != nil {
		nameParam = fmt.Sprintf("%s (%s)", endpoint.Name, project.Name)
		targetURL = endpoint.Url
		failureRateStr = fmt.Sprintf("%.2f", failureRate)
	}

	// The owner is reached by email only; webhook channels are per endpoint.
	return notification.DispatchEndpointAlert(ctx, q, lo, notification.EndpointAlert{
		EmailRecipient: ownerEmail,
		EmailSubject:   "Project Endpoint Disabled - Circuit Breaker Triggered",
		EmailParams: map[string]string{
			"name":            nameParam,
			"logo_url":        project.LogoURL,
			"target_url":      targetURL,
			"failure_msg":     breakerFailureMsg,
			"response_body":   "",
			"failure_rate":    failureRateStr,
			"status_code":     "0",
			"endpoint_status": string(datastore.InactiveEndpointStatus),
		},
	}) || enqueued
}
