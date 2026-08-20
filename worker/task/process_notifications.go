package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/slack-go/slack"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/email"
	notification "github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/smtp"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/msgpack"
)

var ErrInvalidSlackPayload = errors.New("invalid slack payload")
var ErrInvalidTeamsPayload = errors.New("invalid teams payload")
var ErrTeamsRequestFailed = errors.New("failed to post teams notification")
var ErrInvalidNotificationPayload = errors.New("invalid notification payload")
var ErrInvalidNotificationType = errors.New("invalid notification type")

// teamsResponseReadLimit bounds how much of a Teams webhook error response is
// read. Only enough to make a failure diagnosable; the body is drained so the
// connection can be reused, then discarded.
const teamsResponseReadLimit = 1 << 10

func ProcessNotifications(sc smtp.SmtpClient, dispatcher *net.Dispatcher) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		n := &notification.Notification{}
		err := msgpack.DecodeMsgPack(t.Payload(), &n)
		if err != nil {
			err := json.Unmarshal(t.Payload(), &n)
			if err != nil {
				// If unmarshal fails, try parsing as raw email.Message (backward compatibility)
				np := &email.Message{}
				err := msgpack.DecodeMsgPack(t.Payload(), np)
				if err != nil {
					err := json.Unmarshal(t.Payload(), np)
					if err != nil {
						return ErrInvalidNotificationPayload
					}
				}
				// Successfully parsed as email, process it
				if np.Email != "" {
					newEmail := email.NewEmail(sc)
					err = newEmail.Build(string(np.TemplateName), np.Params)
					if err != nil {
						return err
					}
					return newEmail.Send(np.Email, np.Subject)
				}
				return ErrInvalidNotificationPayload
			}
		}

		// If NotificationType is empty and Payload is nil/empty, try parsing original payload as raw email.Message
		payloadEmpty := n.Payload == nil
		if !payloadEmpty {
			// Check if payload is an empty map/interface
			if payloadMap, ok := n.Payload.(map[string]interface{}); ok && len(payloadMap) == 0 {
				payloadEmpty = true
			}
		}
		if n.NotificationType == "" && payloadEmpty {
			np := &email.Message{}
			err := msgpack.DecodeMsgPack(t.Payload(), np)
			if err != nil {
				err := json.Unmarshal(t.Payload(), np)
				if err != nil {
					return ErrInvalidNotificationPayload
				}
			}
			// Successfully parsed as email, process it
			if np.Email != "" {
				newEmail := email.NewEmail(sc)
				err = newEmail.Build(string(np.TemplateName), np.Params)
				if err != nil {
					return err
				}
				return newEmail.Send(np.Email, np.Subject)
			}
			return ErrInvalidNotificationPayload
		}

		bufP, err := json.Marshal(n.Payload)
		if err != nil {
			return err
		}

		switch n.NotificationType {
		case notification.EmailNotificationType:
			np := &email.Message{}
			err := json.Unmarshal(bufP, np)
			if err != nil {
				return ErrInvalidEmailPayload
			}

			newEmail := email.NewEmail(sc)
			err = newEmail.Build(string(np.TemplateName), np.Params)
			if err != nil {
				return err
			}

			return newEmail.Send(np.Email, np.Subject)
		case notification.SlackNotificationType:
			np := &notification.SlackNotification{}
			err := json.Unmarshal(bufP, np)
			if err != nil {
				return ErrInvalidSlackPayload
			}

			convoyAgent := fmt.Sprintf("Convoy/%s", convoy.GetVersion())
			attachment := slack.Attachment{
				AuthorName: convoyAgent,
				Text:       np.Text,
				Ts:         json.Number(strconv.FormatInt(time.Now().Unix(), 10)),
			}

			msg := &slack.WebhookMessage{
				Attachments: []slack.Attachment{attachment},
			}

			// Send through the notification client, which carries an
			// unconditional connect-time SSRF guard, so a user-controlled
			// slack_webhook_url cannot reach internal/private addresses even on
			// deployments without the IpRules license. Default
			// slack.PostWebhookContext uses http.DefaultClient, which has no
			// egress protection at all.
			err = slack.PostWebhookCustomHTTPContext(ctx, np.WebhookURL, dispatcher.NotificationHTTPClient(), msg)
			if err != nil {
				return err
			}
			return nil
		case notification.TeamsNotificationType:
			np := &notification.TeamsNotification{}
			err := json.Unmarshal(bufP, np)
			if err != nil {
				return ErrInvalidTeamsPayload
			}

			return postTeamsCard(ctx, dispatcher.NotificationHTTPClient(), np.WebhookURL, np.Text)

		default:
			// Default to email if notification type is empty/invalid but payload can be parsed as email
			np := &email.Message{}
			err := json.Unmarshal(bufP, np)
			if err == nil && np.Email != "" {
				// Successfully parsed as email, process it
				newEmail := email.NewEmail(sc)
				err = newEmail.Build(string(np.TemplateName), np.Params)
				if err != nil {
					return err
				}
				return newEmail.Send(np.Email, np.Subject)
			}

			return ErrInvalidNotificationType
		}
	}
}

// postTeamsCard posts an Adaptive Card to a Microsoft Teams incoming webhook.
//
// No error returned here may carry webhookURL. A Workflows webhook URL holds its
// authorisation in a `sig` query parameter, so it is bearer-equivalent, and the
// transport errors from http.Client are *url.Error values that quote the URL
// they failed on. Every path below therefore reports a status code or a fixed
// category from classifyTeamsTransportError, never the wrapped error's text,
// and the returned error is what asynq writes to the worker log.
func postTeamsCard(ctx context.Context, client *http.Client, webhookURL, text string) error {
	body, err := json.Marshal(notification.BuildTeamsAdaptiveCard(text))
	if err != nil {
		return fmt.Errorf("failed to encode teams card: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		// The stored URL passed ValidateOutboundURL on write, so this is a fixed
		// category rather than a parse detail, which would quote the URL.
		return fmt.Errorf("%w: invalid webhook url", ErrTeamsRequestFailed)
	}
	req.Header.Set("Content-Type", "application/json")

	// The notification client carries the same unconditional connect-time SSRF
	// guard the Slack leg relies on, because teams_webhook_url is equally
	// user-controlled.
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTeamsRequestFailed, classifyTeamsTransportError(err))
	}
	defer res.Body.Close()

	// Drain a bounded prefix so the connection can be reused, and cap the read so
	// a hostile or misconfigured host cannot stream an unbounded body into the
	// worker. The body itself is discarded rather than reported: a provider error
	// message can quote the request URL, which would put `sig` in the log.
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, teamsResponseReadLimit))

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return fmt.Errorf("%w: status %d", ErrTeamsRequestFailed, res.StatusCode)
	}

	return nil
}
