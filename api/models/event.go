package models

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

type CreateEvent struct {
	UID string `json:"uid" swaggerignore:"true"`

	// Deprecated but necessary for backward compatibility.
	AppID string `json:"app_id"` // Deprecated but necessary for backward compatibility

	// Specifies the endpoint to send this event to.
	EndpointID string `json:"endpoint_id"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" valid:"required~please provide your data" swaggertype:"object"`

	// Event Type is used for filtering and debugging e.g invoice.paid
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	// Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders map[string]string `json:"custom_headers"`

	// Specify a key for event deduplication
	IdempotencyKey string `json:"idempotency_key"`
}

func (e *CreateEvent) Validate() error {
	return util.Validate(e)
}

type DynamicEvent struct {
	// URL is the endpoint's URL prefixed with https. non-https urls are currently
	// not supported.
	URL string `json:"url" valid:"required~please provide a url"`

	// Endpoint's webhook secret. If not provided, Convoy autogenerates one for the endpoint.
	Secret string `json:"secret" valid:"required~please provide a secret"`

	// A list of event types for the subscription filter config
	EventTypes []string `json:"event_types"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" valid:"required~please provide your data"`

	ProjectID string `json:"project_id" swaggerignore:"true"`

	// Event Type is used for filtering and debugging e.g invoice.paid
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	// Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders map[string]string `json:"custom_headers"`

	// Specify a key for event deduplication
	IdempotencyKey string `json:"idempotency_key"`

	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
}

func (de *DynamicEvent) Validate() error {
	return util.Validate(de)
}

type SearchParams struct {
	// The start date
	StartDate string `json:"startDate" example:"2006-01-02T15:04:05"`
	// The end date
	EndDate string `json:"endDate" example:"2008-05-02T15:04:05"`
}

type QueryListEvent struct {
	// Any arbitrary value to filter the events payload
	Query string `json:"query"`

	// A list of Source IDs to filter the events by.
	SourceIDs []string `json:"sourceId"`

	// IdempotencyKey to filter by
	IdempotencyKey string `json:"idempotencyKey"`

	// A list of endpoint ids to filter by
	EndpointIDs []string `json:"endpointId"`

	SearchParams
	Pageable
}

type QueryListEventResponse struct {
	*datastore.Filter
}

func (qs *QueryListEvent) Transform(r *http.Request) (*QueryListEventResponse, error) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		return nil, err
	}

	return &QueryListEventResponse{
		Filter: &datastore.Filter{
			Query:          r.URL.Query().Get("query"),
			IdempotencyKey: r.URL.Query().Get("idempotencyKey"),
			EndpointIDs:    getEndpointIDs(r),
			SourceIDs:      getSourceIDs(r),
			SearchParams:   searchParams,
			Pageable:       m.GetPageableFromContext(r.Context()),
		},
	}, nil
}

type DynamicEventStub struct {
	ProjectID string `json:"project_id"`
	EventType string `json:"event_type" valid:"required~please provide an event type"`
	// Data is an arbitrary JSON value that gets sent as the body of the webhook to the endpoints
	Data           json.RawMessage   `json:"data" valid:"required~please provide your data"`
	CustomHeaders  map[string]string `json:"custom_headers"`
	IdempotencyKey string            `json:"idempotency_key"`
}

func (ds *DynamicEventStub) Validate() error {
	return util.Validate(ds)
}

type BroadcastEvent struct {
	JobID string `json:"jid" swaggerignore:"true"`

	// Event Type is used for filtering and debugging e.g invoice.paid
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	ProjectID string `json:"project_id" swaggerignore:"true"`
	SourceID  string `json:"source_id" swaggerignore:"true"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" valid:"required~please provide your data"`

	// Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders map[string]string `json:"custom_headers"`

	// Specify a key for event deduplication
	IdempotencyKey string `json:"idempotency_key"`

	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
}

func (bs *BroadcastEvent) Validate() error {
	return util.Validate(bs)
}

type FanoutEvent struct {
	// Used for fanout, sends this event to all endpoints with this OwnerID.
	OwnerID string `json:"owner_id" valid:"required~please provide an owner id"`

	// Event Type is used for filtering and debugging e.g invoice.paid
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" valid:"required~please provide your data"`

	// Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders map[string]string `json:"custom_headers"`

	// Specify a key for event deduplication
	IdempotencyKey string `json:"idempotency_key"`
}

func (fe *FanoutEvent) Validate() error {
	return util.Validate(fe)
}

type EventResponse struct {
	*datastore.Event
}

type QueryCountAffectedEvents struct {
	SourceID   string `json:"sourceId"`
	EndpointID string `json:"endpointId"`
	SearchParams
}

type QueryCountAffectedEventsResponse struct {
	*datastore.Filter
}

func (qc *QueryCountAffectedEvents) Transform(r *http.Request) (*QueryCountAffectedEventsResponse, error) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		return nil, err
	}

	return &QueryCountAffectedEventsResponse{
		Filter: &datastore.Filter{
			Pageable:     defaultPageable,
			SourceID:     r.URL.Query().Get("sourceId"),
			EndpointID:   r.URL.Query().Get("endpointId"),
			SearchParams: searchParams,
		},
	}, nil
}

func getEndpointIDs(r *http.Request) []string {
	var endpoints []string

	for _, id := range r.URL.Query()["endpointId"] {
		if !util.IsStringEmpty(id) {
			endpoints = append(endpoints, id)
		}
	}

	return endpoints
}

func getSourceIDs(r *http.Request) []string {
	var sources []string

	for _, id := range r.URL.Query()["sourceId"] {
		if !util.IsStringEmpty(id) {
			sources = append(sources, id)
		}
	}

	return sources
}

func getSearchParams(r *http.Request) (datastore.SearchParams, error) {
	var searchParams datastore.SearchParams
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	var err error

	var startT time.Time
	if len(startDate) == 0 {
		startT = time.Unix(0, 0)
	} else {
		startT, err = time.Parse(format, startDate)
		if err != nil {
			return searchParams, errors.New("please specify a startDate in the format " + format)
		}
	}
	var endT time.Time
	if len(endDate) == 0 {
		now := time.Now()
		endT = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
	} else {
		endT, err = time.Parse(format, endDate)
		if err != nil {
			return searchParams, errors.New("please specify a correct endDate in the format " + format + " or none at all")
		}
	}

	if err := m.EnsurePeriod(startT, endT); err != nil {
		return searchParams, err
	}

	searchParams = datastore.SearchParams{
		CreatedAtStart: startT.Unix(),
		CreatedAtEnd:   endT.Unix(),
	}

	return searchParams, nil
}
