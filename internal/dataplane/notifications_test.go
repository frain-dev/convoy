package dataplane

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	notification "github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

// teamsWebhookURL mirrors the shape of a Power Automate Workflows webhook,
// including the bearer-equivalent sig query parameter.
const teamsWebhookURL = "https://example.logic.azure.com:443/workflows/abc/triggers/manual/paths/invoke?sig=redacted"

// testQueue records what was enqueued. A non-nil writeErr fails every write.
type testQueue struct {
	wrote    []*queue.Job
	writeErr error
}

func (tq *testQueue) write(job *queue.Job) error {
	if tq.writeErr != nil {
		return tq.writeErr
	}
	tq.wrote = append(tq.wrote, job)
	return nil
}

func (tq *testQueue) Write(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
	return tq.write(job)
}

func (tq *testQueue) WriteWithoutTimeout(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
	return tq.write(job)
}

func (tq *testQueue) Options() queue.QueueOptions { return queue.QueueOptions{} }

// notificationTypes lists the type of every enqueued notification, in order.
func notificationTypes(t *testing.T, jobs []*queue.Job) []notification.NotificationType {
	t.Helper()

	types := make([]notification.NotificationType, 0, len(jobs))
	for _, job := range jobs {
		n := &notification.Notification{}
		require.NoError(t, msgpack.DecodeMsgPack(job.Payload, n))
		types = append(types, n.NotificationType)
	}

	return types
}

// webhookText decodes the alert text of the single notification of the given
// webhook type among the jobs. Slack and Teams carry the same payload shape.
func webhookText(t *testing.T, jobs []*queue.Job, want notification.NotificationType) string {
	t.Helper()

	for _, job := range jobs {
		n := &notification.Notification{}
		require.NoError(t, msgpack.DecodeMsgPack(job.Payload, n))
		if n.NotificationType != want {
			continue
		}

		buf, err := json.Marshal(n.Payload)
		require.NoError(t, err)

		payload := &struct {
			Text string `json:"text"`
		}{}
		require.NoError(t, json.Unmarshal(buf, payload))

		return payload.Text
	}

	t.Fatalf("no %s notification was enqueued", want)
	return ""
}

func advancedEndpointLicenser(t *testing.T, enabled bool) license.Licenser {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(enabled)
	return licenser
}

func TestEnqueueCircuitBreakerNotifications(t *testing.T) {
	lo := log.New("convoy", log.LevelError)
	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}

	tests := []struct {
		name       string
		endpoint   *datastore.Endpoint
		ownerEmail string
		wantTypes  []notification.NotificationType
	}{
		{
			name:       "endpoint email and owner email",
			endpoint:   &datastore.Endpoint{Name: "E1", Url: "https://e1.example.com", SupportEmail: "support@example.com"},
			ownerEmail: "owner@example.com",
			wantTypes:  []notification.NotificationType{notification.EmailNotificationType, notification.EmailNotificationType},
		},
		{
			name:       "endpoint slack only, no owner",
			endpoint:   &datastore.Endpoint{Name: "E2", Url: "https://e2.example.com", SlackWebhookURL: "https://hooks.example.com/services/T/B/X"},
			wantTypes:  []notification.NotificationType{notification.SlackNotificationType},
			ownerEmail: "",
		},
		{
			name:       "endpoint teams only, no owner",
			endpoint:   &datastore.Endpoint{Name: "E6", Url: "https://e6.example.com", TeamsWebhookURL: teamsWebhookURL},
			wantTypes:  []notification.NotificationType{notification.TeamsNotificationType},
			ownerEmail: "",
		},
		{
			name: "endpoint email and slack plus owner email",
			endpoint: &datastore.Endpoint{
				Name: "E3", Url: "https://e3.example.com",
				SupportEmail:    "support@example.com",
				SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
			},
			ownerEmail: "owner@example.com",
			wantTypes: []notification.NotificationType{
				notification.EmailNotificationType,
				notification.SlackNotificationType,
				notification.EmailNotificationType,
			},
		},
		{
			name: "every endpoint channel plus owner email",
			endpoint: &datastore.Endpoint{
				Name: "E7", Url: "https://e7.example.com",
				SupportEmail:    "support@example.com",
				SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
				TeamsWebhookURL: teamsWebhookURL,
			},
			ownerEmail: "owner@example.com",
			wantTypes: []notification.NotificationType{
				notification.EmailNotificationType,
				notification.SlackNotificationType,
				notification.TeamsNotificationType,
				notification.EmailNotificationType,
			},
		},
		{
			name:       "no channel configured",
			endpoint:   &datastore.Endpoint{Name: "E4", Url: "https://e4.example.com"},
			ownerEmail: "",
			wantTypes:  []notification.NotificationType{},
		},
		{
			name:       "owner email only",
			endpoint:   &datastore.Endpoint{Name: "E5", Url: "https://e5.example.com"},
			ownerEmail: "owner@example.com",
			wantTypes:  []notification.NotificationType{notification.EmailNotificationType},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			q := &testQueue{}

			sent := EnqueueCircuitBreakerNotifications(context.Background(), q, lo, advancedEndpointLicenser(t, true), project, tc.endpoint, tc.ownerEmail, 42.0)

			require.Equal(t, tc.wantTypes, notificationTypes(t, q.wrote))
			require.Equal(t, len(tc.wantTypes) > 0, sent)
		})
	}
}

// TestEnqueueCircuitBreakerNotifications_AlertText asserts the breaker's webhook
// message stays recognisably about the breaker, not the retry-limit trigger, and
// that Slack and Teams are rendered from the same string.
func TestEnqueueCircuitBreakerNotifications_AlertText(t *testing.T) {
	q := &testQueue{}
	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}
	endpoint := &datastore.Endpoint{
		Name: "E1", Url: "https://e1.example.com",
		SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
		TeamsWebhookURL: teamsWebhookURL,
	}

	sent := EnqueueCircuitBreakerNotifications(context.Background(), q, log.New("convoy", log.LevelError), advancedEndpointLicenser(t, true), project, endpoint, "", 42.5)
	require.True(t, sent)

	slack := webhookText(t, q.wrote, notification.SlackNotificationType)
	require.Contains(t, slack, "Circuit breaker threshold exceeded")
	require.Contains(t, slack, "42.50%")
	require.Contains(t, slack, endpoint.Url)
	require.Contains(t, slack, string(datastore.InactiveEndpointStatus))

	require.Equal(t, slack, webhookText(t, q.wrote, notification.TeamsNotificationType))
}

// TestEnqueueCircuitBreakerNotifications_NilEndpoint covers the defensive nil
// endpoint path: the owner is still notified and nothing dereferences the nil.
func TestEnqueueCircuitBreakerNotifications_NilEndpoint(t *testing.T) {
	q := &testQueue{}
	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}

	sent := EnqueueCircuitBreakerNotifications(context.Background(), q, log.New("convoy", log.LevelError), advancedEndpointLicenser(t, true), project, nil, "owner@example.com", 0)
	require.True(t, sent)

	require.Equal(t, []notification.NotificationType{notification.EmailNotificationType}, notificationTypes(t, q.wrote))
}

// TestEnqueueCircuitBreakerNotifications_QueueFailureIsSurvivable covers a queue
// that rejects every write. Nothing is enqueued and nothing panics; the breaker
// still disabled the endpoint.
func TestEnqueueCircuitBreakerNotifications_QueueFailureIsSurvivable(t *testing.T) {
	lo := log.New("convoy", log.LevelError)
	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}
	endpoint := &datastore.Endpoint{
		Name: "E1", Url: "https://e1.example.com",
		SupportEmail:    "support@example.com",
		SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
		TeamsWebhookURL: teamsWebhookURL,
	}

	broken := &testQueue{writeErr: errors.New("queue unavailable")}
	sent := EnqueueCircuitBreakerNotifications(context.Background(), broken, lo, advancedEndpointLicenser(t, true), project, endpoint, "owner@example.com", 10)
	require.Empty(t, broken.wrote)
	require.False(t, sent)
}

func TestEnqueueCircuitBreakerNotifications_NoDispatchReturnsFalse(t *testing.T) {
	q := &testQueue{}
	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}
	endpoint := &datastore.Endpoint{Name: "E4", Url: "https://e4.example.com"}

	sent := EnqueueCircuitBreakerNotifications(context.Background(), q, log.New("convoy", log.LevelError), advancedEndpointLicenser(t, true), project, endpoint, "", 10)
	require.Empty(t, q.wrote)
	require.False(t, sent)
}

func TestEnqueueCircuitBreakerNotifications_SkipsEndpointChannelsWithoutLicense(t *testing.T) {
	q := &testQueue{}
	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}
	endpoint := &datastore.Endpoint{
		Name: "E1", Url: "https://e1.example.com",
		SupportEmail:    "support@example.com",
		SlackWebhookURL: "https://hooks.example.com/services/T/B/X",
		TeamsWebhookURL: teamsWebhookURL,
	}

	sent := EnqueueCircuitBreakerNotifications(context.Background(), q, log.New("convoy", log.LevelError), advancedEndpointLicenser(t, false), project, endpoint, "owner@example.com", 10)
	require.True(t, sent)

	require.Equal(t, []notification.NotificationType{notification.EmailNotificationType}, notificationTypes(t, q.wrote))
}
