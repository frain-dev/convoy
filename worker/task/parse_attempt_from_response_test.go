package task

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	convoynet "github.com/frain-dev/convoy/net"
)

func newTestResponse(statusCode int) *convoynet.Response {
	u, _ := url.Parse("https://example.com/webhook")
	return &convoynet.Response{
		Status:         http.StatusText(statusCode),
		StatusCode:     statusCode,
		Method:         http.MethodPost,
		URL:            u,
		RequestHeader:  http.Header{},
		ResponseHeader: http.Header{},
		Body:           []byte(`{"ok":true}`),
		IP:             "192.0.2.1",
	}
}

func TestParseAttemptFromResponse_SuccessHasBothTimestamps(t *testing.T) {
	ed := &datastore.EventDelivery{UID: "ed-1", ProjectID: "proj-1"}
	endpoint := &datastore.Endpoint{UID: "ep-1"}
	resp := newTestResponse(http.StatusOK)

	requestedAt := time.Now()
	respondedAt := requestedAt.Add(50 * time.Millisecond)

	attempt := parseAttemptFromResponse(ed, endpoint, resp, true, requestedAt, respondedAt)

	require.True(t, attempt.RequestedAt.Valid)
	require.True(t, attempt.RespondedAt.Valid)
	require.True(t, attempt.RespondedAt.Time.Equal(respondedAt) || attempt.RespondedAt.Time.After(requestedAt))
	require.False(t, attempt.RespondedAt.Time.Before(attempt.RequestedAt.Time))
}

func TestParseAttemptFromResponse_GotResponseFailureHasBothTimestamps(t *testing.T) {
	ed := &datastore.EventDelivery{UID: "ed-2", ProjectID: "proj-1"}
	endpoint := &datastore.Endpoint{UID: "ep-1"}
	resp := newTestResponse(http.StatusInternalServerError)

	requestedAt := time.Now()
	respondedAt := requestedAt.Add(10 * time.Millisecond)

	attempt := parseAttemptFromResponse(ed, endpoint, resp, false, requestedAt, respondedAt)

	require.True(t, attempt.RequestedAt.Valid)
	require.True(t, attempt.RespondedAt.Valid)
	require.False(t, attempt.RespondedAt.Time.Before(attempt.RequestedAt.Time))
}

func TestParseAttemptFromResponse_NoResponseLeavesRespondedAtNull(t *testing.T) {
	ed := &datastore.EventDelivery{UID: "ed-3", ProjectID: "proj-1"}
	endpoint := &datastore.Endpoint{UID: "ep-1"}
	// No HTTP response: status code 0, respondedAt zero value.
	resp := newTestResponse(0)
	resp.Error = "dial tcp: connection refused"

	requestedAt := time.Now()

	attempt := parseAttemptFromResponse(ed, endpoint, resp, false, requestedAt, time.Time{})

	require.True(t, attempt.RequestedAt.Valid)
	require.False(t, attempt.RespondedAt.Valid)
}
