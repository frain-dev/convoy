package notifications

import (
	"context"
	"fmt"
	"strconv"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type NotificationType string

const (
	SlackNotificationType NotificationType = "slack"
	EmailNotificationType NotificationType = "email"
	TeamsNotificationType NotificationType = "teams"
)

// failureRateValue returns the failure rate or 0 when it was not computed (nil).
func failureRateValue(rate *float64) float64 {
	if rate == nil {
		return 0
	}
	return *rate
}

type Notification struct {
	// Defines the type of notification either slack or email.
	NotificationType NotificationType `json:"notification_type,omitempty"`

	// Email or Slack notification
	Payload interface{} `json:"payload,omitempty"`
}

type SlackNotification struct {
	WebhookURL string `json:"webhook_url,omitempty"`

	Text string `json:"text,omitempty"`
}

type TeamsNotification struct {
	WebhookURL string `json:"webhook_url,omitempty"`

	Text string `json:"text,omitempty"`
}

// EndpointAlert is a single endpoint alert to fan out across every channel the
// alert has configured. Callers own the wording for their trigger; this type owns
// channel selection, payload encoding and enqueueing, so a new trigger does not
// have to rebuild the Slack or email payload shape.
type EndpointAlert struct {
	// EmailRecipient is the address for the email channel. Empty skips email.
	EmailRecipient string

	// SlackWebhookURL is the target for the Slack channel. Empty skips Slack.
	SlackWebhookURL string

	// TeamsWebhookURL is the target for the Microsoft Teams channel. Empty skips
	// Teams.
	TeamsWebhookURL string

	// EmailSubject is the subject line of the email channel message.
	EmailSubject string

	// EmailParams fills the endpoint update email template.
	EmailParams map[string]string

	// AlertText is the message body for every webhook channel: the Slack
	// attachment text and the Teams card TextBlock are rendered from this one
	// string, so the two channels cannot drift in wording.
	AlertText string
}

// DispatchEndpointAlert enqueues one notification job per channel the alert has
// configured. Channels are independent: an encode or enqueue failure on one
// channel is logged and the remaining channels are still attempted, so a broken
// Slack or Teams webhook can never suppress the email, or each other.
// Returns true when at least one channel job was written to the queue.
func DispatchEndpointAlert(ctx context.Context, q queue.Queuer, logger log.Logger, alert EndpointAlert) bool {
	var enqueued bool

	// Every endpoint alert trigger renders the same endpoint update template.
	if !util.IsStringEmpty(alert.EmailRecipient) {
		enqueued = enqueueNotification(ctx, q, logger, &Notification{
			NotificationType: EmailNotificationType,
			Payload: email.Message{
				Email:        alert.EmailRecipient,
				Subject:      alert.EmailSubject,
				TemplateName: email.TemplateEndpointUpdate,
				Params:       alert.EmailParams,
			},
		}) || enqueued
	}

	if !util.IsStringEmpty(alert.SlackWebhookURL) {
		enqueued = enqueueNotification(ctx, q, logger, &Notification{
			NotificationType: SlackNotificationType,
			Payload: SlackNotification{
				WebhookURL: alert.SlackWebhookURL,
				Text:       alert.AlertText,
			},
		}) || enqueued
	}

	if !util.IsStringEmpty(alert.TeamsWebhookURL) {
		enqueued = enqueueNotification(ctx, q, logger, &Notification{
			NotificationType: TeamsNotificationType,
			Payload: TeamsNotification{
				WebhookURL: alert.TeamsWebhookURL,
				Text:       alert.AlertText,
			},
		}) || enqueued
	}

	return enqueued
}

// enqueueNotification writes one notification job to the queue. The webhook legs
// are consumed by the notification processor, which posts through the SSRF-guarded
// notification HTTP client; producers never dial the webhook URL themselves.
func enqueueNotification(ctx context.Context, q queue.Queuer, logger log.Logger, n *Notification) bool {
	buf, err := msgpack.EncodeMsgPack(n)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to marshal %v notification payload: %v", n.NotificationType, err))
		return false
	}

	job := &queue.Job{
		ID:      ulid.Make().String(),
		Payload: buf,
	}

	if err = q.Write(ctx, convoy.NotificationProcessor, convoy.DefaultQueue, job); err != nil {
		logger.Error("Failed to write new notification to the queue", "error", err)
		return false
	}

	return true
}

// NOTIFICATIONS

// SendEndpointNotification tells the endpoint's channels that its status changed,
// either because the retry limit disabled it (failure) or because it was
// reactivated. Per-channel failures are logged rather than returned: one broken
// webhook must not suppress the other channels, and the status change stands
// whether or not the alert got out.
func SendEndpointNotification(
	ctx context.Context,
	endpoint *datastore.Endpoint,
	project *datastore.Project,
	status datastore.EndpointStatus,
	q queue.Queuer,
	failure bool,
	failureMsg string,
	responseBody string,
	statusCode int,
	logger log.Logger,
) {
	var alertText string
	if failure {
		alertText = fmt.Sprintf("failed to send event delivery to endpoint url (%s) after retry limit was hit,"+
			" endpoint response body (%s) and status code was %d, reason for failure is %q, endpoint status is now %s",
			endpoint.Url, responseBody, statusCode, failureMsg, status)
	} else {
		alertText = fmt.Sprintf("endpoint url (%s) which was formerly dectivated has now been reactivated, endpoint status is now %s", endpoint.Url, status)
	}

	DispatchEndpointAlert(ctx, q, logger, EndpointAlert{
		EmailRecipient:  endpoint.SupportEmail,
		SlackWebhookURL: endpoint.SlackWebhookURL,
		TeamsWebhookURL: endpoint.TeamsWebhookURL,
		EmailSubject:    "Endpoint Status Update",
		EmailParams: map[string]string{
			"name":            endpoint.Name,
			"logo_url":        project.LogoURL,
			"target_url":      endpoint.Url,
			"failure_msg":     failureMsg,
			"response_body":   responseBody,
			"failure_rate":    fmt.Sprintf("%.2f", failureRateValue(endpoint.FailureRate)),
			"status_code":     strconv.Itoa(statusCode),
			"endpoint_status": string(status),
		},
		AlertText: alertText,
	})
}
