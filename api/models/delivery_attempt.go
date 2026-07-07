package models

import (
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
)

type DeliveryAttemptResponse struct {
	*datastore.DeliveryAttempt
}

// MarshalJSON redacts sensitive header values (auth tokens, API keys, cookies)
// on both the outbound request headers and the endpoint's response headers
// (e.g. Set-Cookie) before serializing a delivery attempt to an API client. It
// operates on a shallow copy with fresh redacted maps, so the stored attempt
// keeps its real values; only the response view is masked.
func (d DeliveryAttemptResponse) MarshalJSON() ([]byte, error) {
	if d.DeliveryAttempt == nil {
		return []byte("null"), nil
	}

	clone := *d.DeliveryAttempt
	clone.RequestHeader = m.RedactSensitiveHeaders(clone.RequestHeader)
	clone.ResponseHeader = m.RedactSensitiveHeaders(clone.ResponseHeader)

	return json.Marshal(&clone)
}

// NewDeliveryAttemptResponses wraps a slice of delivery attempts so each one is
// header-redacted on serialization.
func NewDeliveryAttemptResponses(attempts []datastore.DeliveryAttempt) []DeliveryAttemptResponse {
	responses := make([]DeliveryAttemptResponse, len(attempts))
	for i := range attempts {
		responses[i] = DeliveryAttemptResponse{DeliveryAttempt: &attempts[i]}
	}
	return responses
}
