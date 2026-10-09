package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

// teamsWebhookURL mirrors the shape of a Power Automate Workflows webhook,
// including the bearer-equivalent sig query parameter.
const teamsWebhookURL = "https://example.logic.azure.com:443/workflows/abc/triggers/manual/paths/invoke?sig=redacted"

type testQueue struct {
	wrote   []*queue.Job
	writeFn func(job *queue.Job) error
}

func (tq *testQueue) Write(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
	if tq.writeFn != nil {
		if err := tq.writeFn(job); err != nil {
			return err
		}
	}
	tq.wrote = append(tq.wrote, job)
	return nil
}

func (tq *testQueue) WriteWithoutTimeout(ctx context.Context, name convoy.TaskName, queueName convoy.QueueName, job *queue.Job) error {
	return tq.Write(ctx, name, queueName, job)
}

func (tq *testQueue) Options() queue.QueueOptions { return queue.QueueOptions{} }

// decodeJobs decodes every enqueued job into its notification, keyed by type.
func decodeJobs(t *testing.T, jobs []*queue.Job) map[NotificationType]*Notification {
	t.Helper()

	decoded := make(map[NotificationType]*Notification, len(jobs))
	for _, job := range jobs {
		n := &Notification{}
		require.NoError(t, msgpack.DecodeMsgPack(job.Payload, n))
		decoded[n.NotificationType] = n
	}

	return decoded
}

// slackPayload re-reads the Slack payload the way the notification processor
// does: msgpack decodes into a generic map, which is then re-marshalled as JSON.
func slackPayload(n *Notification) (*SlackNotification, error) {
	buf, err := json.Marshal(n.Payload)
	if err != nil {
		return nil, err
	}

	payload := &SlackNotification{}
	if err = json.Unmarshal(buf, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func teamsPayload(n *Notification) (*TeamsNotification, error) {
	buf, err := json.Marshal(n.Payload)
	if err != nil {
		return nil, err
	}

	payload := &TeamsNotification{}
	if err = json.Unmarshal(buf, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func emailPayload(n *Notification) (*email.Message, error) {
	buf, err := json.Marshal(n.Payload)
	if err != nil {
		return nil, err
	}

	payload := &email.Message{}
	if err = json.Unmarshal(buf, payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func TestDispatchEndpointAlert(t *testing.T) {
	lo := log.New("convoy", log.LevelError)

	tests := []struct {
		name      string
		alert     EndpointAlert
		wantTypes []NotificationType
	}{
		{
			name: "email only",
			alert: EndpointAlert{
				EmailRecipient: "support@example.com",
				EmailSubject:   "Endpoint Status Update",
				EmailParams:    map[string]string{"name": "E1"},
				AlertText:      "ignored, no webhook configured",
			},
			wantTypes: []NotificationType{EmailNotificationType},
		},
		{
			name: "slack only",
			alert: EndpointAlert{
				SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
				AlertText:       "endpoint disabled",
			},
			wantTypes: []NotificationType{SlackNotificationType},
		},
		{
			name: "teams only",
			alert: EndpointAlert{
				TeamsWebhookURL: teamsWebhookURL,
				AlertText:       "endpoint disabled",
			},
			wantTypes: []NotificationType{TeamsNotificationType},
		},
		{
			name: "email and slack",
			alert: EndpointAlert{
				EmailRecipient:  "support@example.com",
				EmailSubject:    "Endpoint Status Update",
				EmailParams:     map[string]string{"name": "E1"},
				SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
				AlertText:       "endpoint disabled",
			},
			wantTypes: []NotificationType{EmailNotificationType, SlackNotificationType},
		},
		{
			name: "every channel",
			alert: EndpointAlert{
				EmailRecipient:  "support@example.com",
				EmailSubject:    "Endpoint Status Update",
				EmailParams:     map[string]string{"name": "E1"},
				SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
				TeamsWebhookURL: teamsWebhookURL,
				AlertText:       "endpoint disabled",
			},
			wantTypes: []NotificationType{EmailNotificationType, SlackNotificationType, TeamsNotificationType},
		},
		{
			name:      "no channel configured",
			alert:     EndpointAlert{AlertText: "nobody is listening"},
			wantTypes: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &testQueue{}

			DispatchEndpointAlert(context.Background(), q, lo, tc.alert)

			require.Len(t, q.wrote, len(tc.wantTypes))

			decoded := decodeJobs(t, q.wrote)
			for _, want := range tc.wantTypes {
				require.Contains(t, decoded, want)
			}

			if n, ok := decoded[SlackNotificationType]; ok {
				payload, pErr := slackPayload(n)
				require.NoError(t, pErr)
				require.Equal(t, tc.alert.SlackWebhookURL, payload.WebhookURL)
				require.Equal(t, tc.alert.AlertText, payload.Text)
			}

			// Both webhook channels render the same AlertText, so a wording change
			// cannot land on one channel only.
			if n, ok := decoded[TeamsNotificationType]; ok {
				payload, pErr := teamsPayload(n)
				require.NoError(t, pErr)
				require.Equal(t, tc.alert.TeamsWebhookURL, payload.WebhookURL)
				require.Equal(t, tc.alert.AlertText, payload.Text)
			}

			if n, ok := decoded[EmailNotificationType]; ok {
				payload, pErr := emailPayload(n)
				require.NoError(t, pErr)
				require.Equal(t, tc.alert.EmailRecipient, payload.Email)
				require.Equal(t, tc.alert.EmailSubject, payload.Subject)
				require.Equal(t, email.TemplateEndpointUpdate, payload.TemplateName)
			}
		})
	}
}

// TestDispatchEndpointAlert_ChannelsAreIndependent asserts the documented failure
// policy across all three legs: a queue write failure on any one channel leaves
// the other two enqueued.
func TestDispatchEndpointAlert_ChannelsAreIndependent(t *testing.T) {
	lo := log.New("convoy", log.LevelError)

	alert := EndpointAlert{
		EmailRecipient:  "support@example.com",
		EmailSubject:    "Endpoint Status Update",
		SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
		TeamsWebhookURL: teamsWebhookURL,
		AlertText:       "endpoint disabled",
	}

	all := []NotificationType{EmailNotificationType, SlackNotificationType, TeamsNotificationType}

	for _, broken := range all {
		t.Run(string(broken)+" fails", func(t *testing.T) {
			q := &testQueue{writeFn: func(job *queue.Job) error {
				n := &Notification{}
				if err := msgpack.DecodeMsgPack(job.Payload, n); err != nil {
					return err
				}
				if n.NotificationType == broken {
					return errors.New("queue unavailable")
				}
				return nil
			}}

			DispatchEndpointAlert(context.Background(), q, lo, alert)

			require.Len(t, q.wrote, len(all)-1)

			decoded := decodeJobs(t, q.wrote)
			require.NotContains(t, decoded, broken)
			for _, want := range all {
				if want == broken {
					continue
				}
				require.Contains(t, decoded, want)
			}
		})
	}
}

func TestSendEndpointNotification(t *testing.T) {
	lo := log.New("convoy", log.LevelError)
	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}

	tests := []struct {
		name      string
		endpoint  *datastore.Endpoint
		wantTypes []NotificationType
	}{
		{
			name:      "email only",
			endpoint:  &datastore.Endpoint{Name: "E1", Url: "https://e1.example.com", SupportEmail: "support@example.com"},
			wantTypes: []NotificationType{EmailNotificationType},
		},
		{
			name:      "slack only",
			endpoint:  &datastore.Endpoint{Name: "E1", Url: "https://e1.example.com", SlackWebhookURL: "https://hooks.example.com/services/T/B/X"},
			wantTypes: []NotificationType{SlackNotificationType},
		},
		{
			name:      "teams only",
			endpoint:  &datastore.Endpoint{Name: "E1", Url: "https://e1.example.com", TeamsWebhookURL: teamsWebhookURL},
			wantTypes: []NotificationType{TeamsNotificationType},
		},
		{
			name: "email and slack",
			endpoint: &datastore.Endpoint{
				Name: "E1", Url: "https://e1.example.com",
				SupportEmail:    "support@example.com",
				SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
			},
			wantTypes: []NotificationType{EmailNotificationType, SlackNotificationType},
		},
		{
			name: "every channel",
			endpoint: &datastore.Endpoint{
				Name: "E1", Url: "https://e1.example.com",
				SupportEmail:    "support@example.com",
				SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
				TeamsWebhookURL: teamsWebhookURL,
			},
			wantTypes: []NotificationType{EmailNotificationType, SlackNotificationType, TeamsNotificationType},
		},
		{
			name:      "no channel configured",
			endpoint:  &datastore.Endpoint{Name: "E1", Url: "https://e1.example.com"},
			wantTypes: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &testQueue{}

			SendEndpointNotification(context.Background(), tc.endpoint, project,
				datastore.InactiveEndpointStatus, q, true, "connection refused", "", 0, lo)

			require.Len(t, q.wrote, len(tc.wantTypes))

			decoded := decodeJobs(t, q.wrote)
			for _, want := range tc.wantTypes {
				require.Contains(t, decoded, want)
			}

			if n, ok := decoded[SlackNotificationType]; ok {
				payload, pErr := slackPayload(n)
				require.NoError(t, pErr)
				require.Equal(t, tc.endpoint.SlackWebhookURL, payload.WebhookURL)
				require.Contains(t, payload.Text, "after retry limit was hit")
			}

			if n, ok := decoded[TeamsNotificationType]; ok {
				payload, pErr := teamsPayload(n)
				require.NoError(t, pErr)
				require.Equal(t, tc.endpoint.TeamsWebhookURL, payload.WebhookURL)
				require.Contains(t, payload.Text, "after retry limit was hit")
			}
		})
	}
}
