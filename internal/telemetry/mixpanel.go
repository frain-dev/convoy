package telemetry

import (
	"context"
	"os"

	"github.com/mixpanel/mixpanel-go"
)

const (
	mixpanelToken string = "50452b2e0f9390a48c97a06c09e7df79"
)

type mixpanelBackend struct {
	client *mixpanel.ApiClient
}

func NewmixpanelBackend() *mixpanelBackend {
	client := mixpanel.NewApiClient(mixpanelToken, mixpanel.DebugHttpCalls(os.Stdout))
	return &mixpanelBackend{client: client}
}

func (mb *mixpanelBackend) Identify(ctx context.Context, instanceID string) error {
	instance := mixpanel.NewPeopleProperties(instanceID, map[string]any{
		// we can't identify the cloud platform yet.
		"cloud": "none",
	})

	return mb.client.PeopleSet(ctx, []*mixpanel.PeopleProperties{instance})
}

func (mb *mixpanelBackend) Capture(ctx context.Context, metric metric) error {
	return mb.client.Track(ctx, []*mixpanel.Event{
		mb.client.NewEvent(string(metric.Name), metric.InstanceID, map[string]any{
			"Count": metric.Count,
		}),
	})
}
