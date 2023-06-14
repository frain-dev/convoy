package models

import (
	"encoding/json"
	"errors"
	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
	"net/http"
	"time"
)

type CreateEvent struct {
	EndpointID string `json:"endpoint_id"`
	AppID      string `json:"app_id"` // Deprecated but necessary for backward compatibility
	EventType  string `json:"event_type" valid:"required~please provide an event type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data           json.RawMessage   `json:"data" valid:"required~please provide your data"`
	CustomHeaders  map[string]string `json:"custom_headers"`
	IdempotencyKey string            `json:"idempotency_key"`
	IdempotencyTTL string            `json:"idempotency_ttl"`
}

func (e *CreateEvent) Validate() error {
	return util.Validate(e)
}

type DynamicEvent struct {
	Endpoint     DynamicEndpoint     `json:"endpoint"`
	Subscription DynamicSubscription `json:"subscription"`
	Event        DynamicEventStub    `json:"event"`
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
	// Any arbitary value to filter the events payload
	Query    string `json:"query"`
	SourceID string `json:"sourceId"`
	SearchParams
	// A list of endpoint ids to filter by
	EndpointIDs []string `json:"endpointId"`
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
			Query:        r.URL.Query().Get("query"),
			EndpointIDs:  getEndpointIDs(r),
			SourceID:     r.URL.Query().Get("sourceId"),
			SearchParams: searchParams,
			Pageable:     m.GetPageableFromContext(r.Context()),
		},
	}, nil
}

type DynamicEventStub struct {
	ProjectID string `json:"project_id"`
	EventType string `json:"event_type" valid:"required~please provide an event type"`
	// Data is an arbitrary JSON value that gets sent as the body of the webhook to the endpoints
	Data          json.RawMessage   `json:"data" valid:"required~please provide your data"`
	CustomHeaders map[string]string `json:"custom_headers"`
}

func (ds *DynamicEventStub) Validate() error {
	return util.Validate(ds)
}

type FanoutEvent struct {
	OwnerID   string `json:"owner_id" valid:"required~please provide an owner id"`
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data          json.RawMessage   `json:"data" valid:"required~please provide your data"`
	CustomHeaders map[string]string `json:"custom_headers"`
}

func (fe *FanoutEvent) Validate() error {
	return util.Validate(fe)
}

type EventResponse struct {
	*datastore.Event
}

type QueryBatchReplayEvent struct {
	// The endpoint ID to filter by
	EndpointID string `json:"endpointId"`
	// The source ID to filter by
	SourceID string `json:"sourceId"`
	SearchParams
}

type QueryBatchReplayEventResponse struct {
	*datastore.Filter
}

func (qb *QueryBatchReplayEvent) Transform(r *http.Request) (*QueryBatchReplayEventResponse, error) {
	searchParams, err := getSearchParams(r)
	if err != nil {
		return nil, err
	}

	return &QueryBatchReplayEventResponse{
		Filter: &datastore.Filter{
			Pageable:     defaultPageable,
			SourceID:     r.URL.Query().Get("sourceId"),
			EndpointID:   r.URL.Query().Get("endpointId"),
			SearchParams: searchParams,
		},
	}, nil
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
