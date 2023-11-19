package models

import (
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

type EventDeliveryResponse struct {
	*datastore.EventDelivery
}

var defaultPageable datastore.Pageable = datastore.Pageable{
	Direction:  datastore.Next,
	PerPage:    1000000000000,
	NextCursor: datastore.DefaultCursor,
}

type QueryBatchRetryEventDelivery struct {
	SubscriptionID string `json:"subscriptionId"`
	// A list of endpoint IDs to filter by
	EndpointIDs []string `json:"endpointId"`
	EventID     string   `json:"eventId"`
	// A list of event delivery statuses to filter by
	Status []string `json:"status"`
}

type QueryBatchRetryEventDeliveryResponse struct {
	*datastore.Filter
}

func (qb *QueryBatchRetryEventDelivery) Transform(r *http.Request) (*QueryBatchRetryEventDeliveryResponse, error) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		return nil, err
	}

	return &QueryBatchRetryEventDeliveryResponse{
		Filter: &datastore.Filter{
			EndpointIDs:    getEndpointIDs(r),
			SubscriptionID: r.URL.Query().Get("subscriptionId"),
			EventID:        r.URL.Query().Get("eventId"),
			Status:         getEventDeliveryStatus(r),
			Pageable:       defaultPageable,
			SearchParams:   searchParams,
		},
	}, nil
}

type IDs struct {
	IDs []string `json:"ids"`
}

type QueryListEventDelivery struct {
	// A list of endpoint IDs to filter by
	EndpointIDs    []string `json:"endpointId"`
	EventID        string   `json:"eventId"`
	SubscriptionID string   `json:"subscriptionId"`
	IdempotencyKey string   `json:"idempotencyKey"`
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
			EndpointIDs:    getEndpointIDs(r),
			SubscriptionID: r.URL.Query().Get("subscriptionId"),
			IdempotencyKey: r.URL.Query().Get("idempotencyKey"),
			EventID:        r.URL.Query().Get("eventId"),
			EventType:      r.URL.Query().Get("eventType"),
			Status:         getEventDeliveryStatus(r),
			Pageable:       m.GetPageableFromContext(r.Context()),
			SearchParams:   searchParams,
		},
	}, nil
}

type QueryCountAffectedEventDeliveries struct {
	// A list of endpoint IDs to filter by
	EndpointIDs []string `json:"endpointId"`
	EventID     string   `json:"eventId"`
	// A list of event delivery statuses to filter by
	Status []string `json:"status"`
	SearchParams
}

type QueryCountAffectedEventDeliveriesResponse struct {
	*datastore.Filter
}

func (qc *QueryCountAffectedEventDeliveries) Transform(r *http.Request) (*QueryCountAffectedEventDeliveriesResponse, error) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		return nil, err
	}

	return &QueryCountAffectedEventDeliveriesResponse{
		Filter: &datastore.Filter{
			EndpointIDs:  getEndpointIDs(r),
			EventID:      r.URL.Query().Get("eventId"),
			Status:       getEventDeliveryStatus(r),
			SearchParams: searchParams,
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
