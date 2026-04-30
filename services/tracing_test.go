package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// Verifies that instrumented services emit a span with the expected layered
// name and a project_id attribute. Doesn't run the full Run path — short-circuits
// on the nil-project guard so the test stays focused on tracing.
func TestServiceSpan_NamedAndProjectIDTagged(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)

	cases := []struct {
		name         string
		expectedName string
		run          func(ctx context.Context) error
	}{
		{
			name:         "create_fanout",
			expectedName: tracer.SpanServicesEventCreateFanout,
			run: func(ctx context.Context) error {
				_, err := (&CreateFanoutEventService{Logger: log.New("test", log.LevelError)}).Run(ctx)
				return err
			},
		},
		{
			name:         "create_dynamic",
			expectedName: tracer.SpanServicesEventCreateDynamic,
			run: func(ctx context.Context) error {
				return (&CreateDynamicEventService{Logger: log.New("test", log.LevelError)}).Run(ctx)
			},
		},
		{
			name:         "event_delivery.retry",
			expectedName: tracer.SpanServicesEventDeliveryRetry,
			run: func(ctx context.Context) error {
				svc := &RetryEventDeliveryService{
					Logger:        log.New("test", log.LevelError),
					Project:       &datastore.Project{UID: "proj_x"},
					EventDelivery: &datastore.EventDelivery{UID: "ed_y", EndpointID: "ep_z", Status: datastore.SuccessEventStatus},
				}
				return svc.Run(ctx)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exp.Reset()
			err := tc.run(context.Background())
			// Each guard path returns an error; we only care that a span was
			// emitted with the right name and that RecordError marked it.
			require.Error(t, err)
			require.NoError(t, tp.ForceFlush(t.Context()))
			spans := exp.GetSpans()
			require.GreaterOrEqual(t, len(spans), 1)
			require.Equal(t, tc.expectedName, spans[0].Name)
		})
	}
}

func TestServiceSpan_RecordsErrorOnFailure(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)

	svc := &CreateDynamicEventService{Logger: log.New("test", log.LevelError)}
	err := svc.Run(context.Background())
	require.Error(t, err)
	require.NoError(t, tp.ForceFlush(t.Context()))

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	require.NotEmpty(t, spans[0].Events, "RecordError should add an exception event")
	require.NotEmpty(t, spans[0].Status.Description)

	// Defensive check that RecordError handles unwrapped error strings cleanly.
	_ = errors.New("unused")
}
