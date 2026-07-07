package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
)

func TestDeliveryAttemptResponse_MarshalJSON_RedactsRequestHeaders(t *testing.T) {
	attempt := &datastore.DeliveryAttempt{
		UID: "att-1",
		RequestHeader: datastore.HttpHeader{
			"Authorization":      "Bearer live-secret-9999",
			"X-My-Api-Key":       "abc",
			"Content-Type":       "application/json",
			"X-Convoy-Signature": "t=1,v1=abc",
		},
		ResponseHeader: datastore.HttpHeader{
			"Set-Cookie":   "session=live-cookie-8888",
			"Content-Type": "application/json",
		},
	}

	b, err := json.Marshal(DeliveryAttemptResponse{DeliveryAttempt: attempt})
	require.NoError(t, err)

	var out struct {
		RequestHeader  map[string]string `json:"request_http_header"`
		ResponseHeader map[string]string `json:"response_http_header"`
	}
	require.NoError(t, json.Unmarshal(b, &out))

	// Sensitive credentials are fully masked (no trailing bytes leaked).
	require.Equal(t, "***", out.RequestHeader["Authorization"])
	require.Equal(t, "***", out.RequestHeader["X-My-Api-Key"])
	require.Equal(t, "application/json", out.RequestHeader["Content-Type"])
	// Signature is not a credential and stays visible for debugging.
	require.Equal(t, "t=1,v1=abc", out.RequestHeader["X-Convoy-Signature"])

	// Response headers from the endpoint are redacted too (e.g. Set-Cookie).
	require.Equal(t, "***", out.ResponseHeader["Set-Cookie"])
	require.Equal(t, "application/json", out.ResponseHeader["Content-Type"])

	// Underlying attempt is untouched so DB / dispatch values survive.
	require.Equal(t, "Bearer live-secret-9999", attempt.RequestHeader["Authorization"])
	require.Equal(t, "session=live-cookie-8888", attempt.ResponseHeader["Set-Cookie"])
}

func TestDeliveryAttemptResponse_MarshalJSON_RawHeadersForTrustedCaller(t *testing.T) {
	attempt := &datastore.DeliveryAttempt{
		UID: "att-1",
		RequestHeader: datastore.HttpHeader{
			"Authorization": "Bearer live-secret-9999",
			"Content-Type":  "application/json",
		},
		ResponseHeader: datastore.HttpHeader{
			"Set-Cookie": "session=live-cookie-8888",
		},
	}

	// showRawHeaders=true (API-key / dashboard caller) returns unmasked values.
	b, err := json.Marshal(NewDeliveryAttemptResponse(attempt, true))
	require.NoError(t, err)

	var out struct {
		RequestHeader  map[string]string `json:"request_http_header"`
		ResponseHeader map[string]string `json:"response_http_header"`
	}
	require.NoError(t, json.Unmarshal(b, &out))

	require.Equal(t, "Bearer live-secret-9999", out.RequestHeader["Authorization"])
	require.Equal(t, "application/json", out.RequestHeader["Content-Type"])
	require.Equal(t, "session=live-cookie-8888", out.ResponseHeader["Set-Cookie"])
}

func TestNewDeliveryAttemptResponses_RedactsByPortalTrust(t *testing.T) {
	attempts := []datastore.DeliveryAttempt{
		{UID: "att-1", RequestHeader: datastore.HttpHeader{"Authorization": "Bearer live-secret-9999"}},
	}

	redacted, err := json.Marshal(NewDeliveryAttemptResponses(attempts, false))
	require.NoError(t, err)
	require.Contains(t, string(redacted), "***")
	require.NotContains(t, string(redacted), "Bearer live-secret-9999")

	raw, err := json.Marshal(NewDeliveryAttemptResponses(attempts, true))
	require.NoError(t, err)
	require.Contains(t, string(raw), "Bearer live-secret-9999")
}

func TestDeliveryAttemptResponse_MarshalJSON_NilIsNull(t *testing.T) {
	b, err := json.Marshal(DeliveryAttemptResponse{})
	require.NoError(t, err)
	require.Equal(t, "null", string(b))
}

func TestEventDeliveryResponse_MarshalJSON_RedactsHeaders(t *testing.T) {
	ed := &datastore.EventDelivery{
		UID: "ed-1",
		Headers: httpheader.HTTPHeader{
			"Authorization":      {"Bearer live-secret-9999"},
			"Content-Type":       {"application/json"},
			"X-Convoy-Signature": {"t=1,v1=abc"},
		},
	}

	b, err := json.Marshal(EventDeliveryResponse{EventDelivery: ed})
	require.NoError(t, err)

	var out struct {
		Headers map[string][]string `json:"headers"`
	}
	require.NoError(t, json.Unmarshal(b, &out))

	require.Equal(t, []string{"***"}, out.Headers["Authorization"])
	require.Equal(t, []string{"application/json"}, out.Headers["Content-Type"])
	require.Equal(t, []string{"t=1,v1=abc"}, out.Headers["X-Convoy-Signature"])

	require.Equal(t, []string{"Bearer live-secret-9999"}, []string(ed.Headers["Authorization"]))
}

func TestEventDeliveryResponse_MarshalJSON_RawHeadersForTrustedCaller(t *testing.T) {
	ed := &datastore.EventDelivery{
		UID: "ed-1",
		Headers: httpheader.HTTPHeader{
			"Authorization": {"Bearer live-secret-9999"},
			"Content-Type":  {"application/json"},
		},
	}

	// showRawHeaders=true (API-key / dashboard caller) returns unmasked values.
	b, err := json.Marshal(NewEventDeliveryResponse(ed, true))
	require.NoError(t, err)

	var out struct {
		Headers map[string][]string `json:"headers"`
	}
	require.NoError(t, json.Unmarshal(b, &out))

	require.Equal(t, []string{"Bearer live-secret-9999"}, out.Headers["Authorization"])
	require.Equal(t, []string{"application/json"}, out.Headers["Content-Type"])
}

func TestEventDeliveryResponse_MarshalJSON_NilIsNull(t *testing.T) {
	b, err := json.Marshal(EventDeliveryResponse{})
	require.NoError(t, err)
	require.Equal(t, "null", string(b))
}
