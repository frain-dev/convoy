package models

import (
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

type DeliveryAttemptResponse struct {
	*datastore.DeliveryAttempt

	// showRawHeaders, when false (the zero value), masks sensitive header values
	// on serialization. It is only set true for higher-trust callers via the
	// constructors, so a bare literal fails closed to redaction.
	showRawHeaders bool
}

// NewDeliveryAttemptResponse wraps a single delivery attempt for API
// serialization. Pass showRawHeaders=true only for callers allowed to see
// unredacted headers (API-key and authenticated dashboard callers).
func NewDeliveryAttemptResponse(attempt *datastore.DeliveryAttempt, showRawHeaders bool) DeliveryAttemptResponse {
	return DeliveryAttemptResponse{DeliveryAttempt: attempt, showRawHeaders: showRawHeaders}
}

// MarshalJSON redacts sensitive header values (auth tokens, API keys, cookies)
// on both the outbound request headers and the endpoint's response headers
// (e.g. Set-Cookie) before serializing a delivery attempt, unless the caller is
// allowed to see raw headers. It operates on a shallow copy with fresh maps, so
// the stored attempt keeps its real values; only the response view is masked.
func (d DeliveryAttemptResponse) MarshalJSON() ([]byte, error) {
	if d.DeliveryAttempt == nil {
		return []byte("null"), nil
	}

	clone := *d.DeliveryAttempt
	if !d.showRawHeaders {
		clone.RequestHeader = m.RedactSensitiveHeaders(clone.RequestHeader)
		clone.ResponseHeader = m.RedactSensitiveHeaders(clone.ResponseHeader)
	}

	return json.Marshal(&clone)
}

// NewDeliveryAttemptResponses wraps a slice of delivery attempts for API
// serialization. Pass showRawHeaders=true only for callers allowed to see
// unredacted headers; otherwise each attempt is header-redacted.
func NewDeliveryAttemptResponses(attempts []datastore.DeliveryAttempt, showRawHeaders bool) []DeliveryAttemptResponse {
	responses := make([]DeliveryAttemptResponse, len(attempts))
	for i := range attempts {
		responses[i] = DeliveryAttemptResponse{DeliveryAttempt: &attempts[i], showRawHeaders: showRawHeaders}
	}
	return responses
}
