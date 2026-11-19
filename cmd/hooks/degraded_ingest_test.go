package hooks

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/pkg/logger"
)

type degradedIngestLog struct {
	logger.Logger
	notice []any
}

func (l *degradedIngestLog) Error(args ...any) {
	l.notice = args
}

func TestDegradedIngestCorrelatesConfiguredSource(t *testing.T) {
	const guid = "00000000-0000-4000-8000-000000000001"
	log := &degradedIngestLog{}
	handler := degradedIngestHandler(config.Configuration{
		RootPath:                "/convoy",
		IngestDeliveryIDHeaders: map[string]string{"source-a": "X-Provider-Delivery"},
	}, true, log)
	req := httptest.NewRequest(http.MethodPost, "/convoy/ingest/source-a", nil)
	req.Header.Set("X-Provider-Delivery", guid)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	require.Equal(t, http.StatusServiceUnavailable, res.Code)
	require.NotEmpty(t, res.Header().Get("X-Request-ID"))
	require.Equal(t, []any{
		"webhook ingestion rejected", "request_id", res.Header().Get("X-Request-ID"),
		"upstream_delivery_id", guid, "http_status", http.StatusServiceUnavailable,
		"reason", "Postgres unavailable during startup",
	}, log.notice)

	log.notice = nil
	req = httptest.NewRequest(http.MethodPost, "/ingest/source-a", nil)
	req.Header.Set("X-Provider-Delivery", guid)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	require.Equal(t, http.StatusServiceUnavailable, res.Code)
	require.Equal(t, "2", res.Header().Get("Retry-After"))
	require.Contains(t, log.notice, guid)

	log.notice = nil
	req = httptest.NewRequest(http.MethodPost, "/convoy/ingest/source-b", nil)
	req.Header.Set("X-Provider-Delivery", guid)
	res = httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	require.Equal(t, http.StatusServiceUnavailable, res.Code)
	require.Empty(t, log.notice)
}
