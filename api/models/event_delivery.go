package models

import (
	"encoding/json"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

type EventDeliveryResponse struct {
	*datastore.EventDelivery
}

// MarshalJSON redacts sensitive request header values (auth tokens, API keys,
// cookies, signatures) before serializing an event delivery to an API client.
// It operates on a shallow copy with a fresh redacted Headers map, so the
// stored delivery and the headers reinjected at dispatch time keep their real
// values; only the response view is masked.
func (e EventDeliveryResponse) MarshalJSON() ([]byte, error) {
	if e.EventDelivery == nil {
		return []byte("null"), nil
	}

	clone := *e.EventDelivery
	clone.Headers = m.RedactSensitiveMultiHeaders(clone.Headers)

	return json.Marshal(&clone)
}

var defaultPageable datastore.Pageable = datastore.Pageable{
	Direction:  datastore.Next,
	PerPage:    1000000000000,
	NextCursor: datastore.DefaultCursor,
}

type IDs struct {
	// A list of event delivery IDs to forcefully resend.
	IDs []string `json:"ids"`
}

type QueryListEventDelivery struct {
	// A list of endpoint IDs to filter by
	EndpointIDs []string `json:"endpointId"`

	// Event ID to filter by
	EventID string `json:"eventId"`

	// SubscriptionID to filter by
	SubscriptionID string `json:"subscriptionId"`

	// IdempotencyKey to filter by
	IdempotencyKey string `json:"idempotencyKey"`

	// EventType to filter by
	EventType string `json:"event_type"`

	// A list of event delivery statuses to filter by
	Status []string `json:"status"`

	SearchParams
	Pageable
}

type QueryListEventDeliveryResponse struct {
	*datastore.Filter
}

func (ql *QueryListEventDelivery) Transform(r *http.Request) (*QueryListEventDeliveryResponse, error) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		return nil, err
	}

	return &QueryListEventDeliveryResponse{
		Filter: &datastore.Filter{
			EndpointIDs:     getEndpointIDs(r),
			SubscriptionID:  r.URL.Query().Get("subscriptionId"),
			IdempotencyKey:  r.URL.Query().Get("idempotencyKey"),
			BrokerMessageId: r.URL.Query().Get("brokerMessageId"),
			EventID:         r.URL.Query().Get("eventId"),
			EventType:       r.URL.Query().Get("eventType"),
			Pageable:        m.GetPageableFromContext(r.Context()),
			Status:          getEventDeliveryStatus(r),
			SearchParams:    searchParams,
		},
	}, nil
}

func getEventDeliveryStatus(r *http.Request) []datastore.EventDeliveryStatus {
	status := make([]datastore.EventDeliveryStatus, 0)

	for _, s := range r.URL.Query()["status"] {
		if !util.IsStringEmpty(s) {
			status = append(status, datastore.EventDeliveryStatus(s))
		}
	}

	return status
}
