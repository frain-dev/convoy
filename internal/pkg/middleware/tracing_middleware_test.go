package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/frain-dev/convoy/internal/pkg/tracer"
)

// Verifies that InstrumentRequests + EnrichSpanFromRoute produce a span named
// after the chi route template and decorated with the conventional convoy.*
// attributes pulled from URL parameters.
func TestInstrumentRequests_EnrichesAndRoutesNamesSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	router := chi.NewMux()
	router.Use(InstrumentRequests("test-server", router, tp))
	router.Use(EnrichSpanFromRoute)
	router.Get("/api/v1/projects/{projectID}/endpoints/{endpointID}/events/{eventID}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := httptest.NewServer(router)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/projects/proj_1/endpoints/ep_2/events/ev_3")
	require.NoError(t, err)
	resp.Body.Close()

	require.NoError(t, tp.ForceFlush(t.Context()))

	spans := exp.GetSpans()
	require.Len(t, spans, 1)

	stub := spans[0]
	require.Contains(t, stub.Name, "/api/v1/projects/{projectID}/endpoints/{endpointID}/events/{eventID}")

	got := map[string]string{}
	for _, kv := range stub.Attributes {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	require.Equal(t, "proj_1", got[string(tracer.AttrProjectID)])
	require.Equal(t, "ep_2", got[string(tracer.AttrEndpointID)])
	require.Equal(t, "ev_3", got[string(tracer.AttrEventID)])
}

// Verifies the filter excludes health/metrics paths from span creation.
func TestInstrumentRequests_FiltersHealthAndMetrics(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	router := chi.NewMux()
	router.Use(InstrumentRequests("test-server", router, tp))
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	router.Get("/metrics", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	router.Get("/api/v1/anything", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	srv := httptest.NewServer(router)
	defer srv.Close()

	for _, p := range []string{"/healthz", "/metrics"} {
		resp, err := http.Get(srv.URL + p)
		require.NoError(t, err)
		resp.Body.Close()
	}
	resp, err := http.Get(srv.URL + "/api/v1/anything")
	require.NoError(t, err)
	resp.Body.Close()

	require.NoError(t, tp.ForceFlush(t.Context()))

	spans := exp.GetSpans()
	require.Len(t, spans, 1)
	require.Contains(t, spans[0].Name, "/api/v1/anything")
}
