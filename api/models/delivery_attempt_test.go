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

	// Long credential reveals only its trailing characters; short one fully masks.
	require.Equal(t, "***9999", out.RequestHeader["Authorization"])
	require.Equal(t, "***", out.RequestHeader["X-My-Api-Key"])
	require.Equal(t, "application/json", out.RequestHeader["Content-Type"])
	// Signature is not a credential and is shown in the dashboard for debugging.
	require.Equal(t, "t=1,v1=abc", out.RequestHeader["X-Convoy-Signature"])

	// Response headers from the endpoint are redacted too (e.g. Set-Cookie).
	require.Equal(t, "***8888", out.ResponseHeader["Set-Cookie"])
	require.Equal(t, "application/json", out.ResponseHeader["Content-Type"])

	// Underlying attempt is untouched so DB / dispatch values survive.
	require.Equal(t, "Bearer live-secret-9999", attempt.RequestHeader["Authorization"])
	require.Equal(t, "session=live-cookie-8888", attempt.ResponseHeader["Set-Cookie"])
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

	require.Equal(t, []string{"***9999"}, out.Headers["Authorization"])
	require.Equal(t, []string{"application/json"}, out.Headers["Content-Type"])
	require.Equal(t, []string{"t=1,v1=abc"}, out.Headers["X-Convoy-Signature"])

	require.Equal(t, []string{"Bearer live-secret-9999"}, []string(ed.Headers["Authorization"]))
}

func TestEventDeliveryResponse_MarshalJSON_NilIsNull(t *testing.T) {
	b, err := json.Marshal(EventDeliveryResponse{})
	require.NoError(t, err)
	require.Equal(t, "null", string(b))
}
