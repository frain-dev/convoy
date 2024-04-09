package telemetry

import (
	"context"
	"github.com/frain-dev/convoy"

	"github.com/posthog/posthog-go"
)

const (
	posthogToken string = "phc_haFlKah9t6DAxdKAopRND4yDps49TRWGeJahMXad4no"
)

type posthogBackend struct {
	client posthog.Client
}

func NewposthogBackend() *posthogBackend {
	client := posthog.New(posthogToken)
	return &posthogBackend{client: client}
}

func (pb *posthogBackend) Identify(ctx context.Context, instanceID string) error {
	return pb.client.Enqueue(
		posthog.Identify{
			DistinctId: instanceID,
			Properties: posthog.NewProperties().
				Set("cloud", "none").
				Set("Version", convoy.GetVersion()),
		})
}

func (pb *posthogBackend) Capture(ctx context.Context, metric Metric) error {
	return pb.client.Enqueue(
		posthog.Capture{
			DistinctId: metric.InstanceID,
			Event:      string(metric.Name),
			Properties: posthog.NewProperties().
				Set("Count", metric.Count).
				Set("Version", convoy.GetVersion()),
		})
}

func (pb *posthogBackend) Close() error {
	return pb.client.Close()
}
