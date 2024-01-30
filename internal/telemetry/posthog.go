package telemetry

import (
	"context"

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
	defer pb.client.Close()

	return pb.client.Enqueue(
		posthog.Identify{
			DistinctId: instanceID,
			Properties: posthog.NewProperties().
				Set("cloud", "none"),
		})
}

func (pb *posthogBackend) Capture(ctx context.Context, metric metric) error {
	defer pb.client.Close()

	return pb.client.Enqueue(
		posthog.Capture{
			DistinctId: metric.InstanceID,
			Event:      string(metric.Name),
			Properties: posthog.NewProperties().
				Set("Count", metric.Count),
		})
}
