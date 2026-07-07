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

	// showRawHeaders, when false (the zero value), masks sensitive header values
	// on serialization. It is only set true for higher-trust callers via
	// NewEventDeliveryResponse, so a bare literal fails closed to redaction.
	showRawHeaders bool
}

// NewEventDeliveryResponse wraps an event delivery for API serialization.
// Pass showRawHeaders=true only for callers allowed to see unredacted headers
// (API-key and authenticated dashboard callers); portal-link callers pass false.
func NewEventDeliveryResponse(ed *datastore.EventDelivery, showRawHeaders bool) EventDeliveryResponse {
	return EventDeliveryResponse{EventDelivery: ed, showRawHeaders: showRawHeaders}
}

// MarshalJSON redacts sensitive request header values (auth tokens, API keys,
// cookies) before serializing an event delivery, unless the caller is allowed
// to see raw headers. It operates on a shallow copy with a fresh Headers map,
// so the stored delivery and the headers reinjected at dispatch time keep their
// real values; only the response view is masked. Webhook signatures stay
// visible either way.
func (e EventDeliveryResponse) MarshalJSON() ([]byte, error) {
	if e.EventDelivery == nil {
		return []byte("null"), nil
	}

	clone := *e.EventDelivery
	if !e.showRawHeaders {
		clone.Headers = m.RedactSensitiveMultiHeaders(clone.Headers)
	}

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
