package postgres

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

func TestMonitorHTMLAndJSON(t *testing.T) {
	q := setupQueue(t)
	ctx := t.Context()
	require.NoError(t, q.Write(ctx, convoy.EventProcessor, convoy.EventQueue, &queue.Job{
		ID:      ulid.Make().String(),
		Payload: []byte("x"),
	}))

	req := httptest.NewRequest(http.MethodGet, "/queue/monitoring/embed/", nil)
	rec := httptest.NewRecorder()
	q.Monitor().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), string(convoy.EventQueue))
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")

	req = httptest.NewRequest(http.MethodGet, "/queue/monitoring/embed/?format=json", nil)
	rec = httptest.NewRecorder()
	q.Monitor().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Queues []QueueCount `json:"queues"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Queues, 1)
	require.Equal(t, int64(1), body.Queues[0].Pending)
}
