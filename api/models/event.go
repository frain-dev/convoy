package models

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/frain-dev/convoy/datastore"
	m "github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/util"
)

type CreateEvent struct {
	UID string `json:"uid" swaggerignore:"true"`

	// Deprecated but necessary for backward compatibility.
	AppID string `json:"app_id"` // Deprecated but necessary for backward compatibility

	// Specifies the endpoint to send this event to. Required unless the
	// deprecated app_id is provided.
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

// requiredField pairs the emptiness of one field with the error util.Validate
// would have produced for it, as "<json name>:<message>".
//
// Emptiness is govalidator's: a zero length value, untrimmed. A whitespace-only
// event type is therefore still accepted here, as it always has been.
type requiredField struct {
	empty   bool
	message string
}

// requiredFieldsError reports every empty required field in one error, joined
// with ", " as util.Validate joins govalidator's. A single missing field
// produces the string it always did; several now come out in field declaration
// order, where util.Validate ranged over a map and so ordered them at random.
//
// The ingest event structs below validate through this rather than through
// util.Validate because govalidator's slice branch runs the whole validator
// machinery once per element, and a json.RawMessage payload is a byte slice, so
// the cost of validating two required fields grew with the size of the webhook
// body. TestIngestEventValidateEnforcesEveryValidTag keeps these checks in step
// with the valid: tags, which stay on the fields as the declaration.
func requiredFieldsError(fields ...requiredField) error {
	var messages []string

	for _, field := range fields {
		if field.empty {
			messages = append(messages, field.message)
		}
	}

	if len(messages) == 0 {
		return nil
	}

	return errors.New(strings.Join(messages, ", "))
}

func (e *CreateEvent) Validate() error {
	if err := requiredFieldsError(
		requiredField{len(e.Data) == 0, "data:please provide your data"},
		requiredField{len(e.EventType) == 0, "event_type:please provide an event type"},
	); err != nil {
		return err
	}

	// An endpoint event must name its delivery target at intake: endpoint_id, or
	// the deprecated app_id alias. Without one the worker can never resolve an
	// endpoint, so the request is rejected here (400) instead of being queued and
	// silently delivering to nothing. The pubsub single-message path enforces the
	// same rule.
	if util.IsStringEmpty(e.EndpointID) && util.IsStringEmpty(e.AppID) {
		return errors.New("please provide an endpoint ID")
	}

	return nil
}

type DynamicEvent struct {
	JobID string `json:"jid" swaggerignore:"true"`

	EventID string `json:"event_id" swaggerignore:"true"`

	// URL is the endpoint's URL prefixed with https. non-https urls are currently
	// not supported.
	URL string `json:"url" valid:"required~please provide an endpoint url"`

	// Endpoint's webhook secret. If not provided, Convoy autogenerates one for the endpoint.
	Secret string `json:"secret" valid:"optional~please provide a secret"`

	// A list of event types for the subscription filter config
	EventTypes []string `json:"event_types"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" valid:"required~please provide your webhook event data" swaggertype:"object"`

	ProjectID string `json:"project_id" swaggerignore:"true"`

	// Event Type is used for filtering and debugging e.g invoice.paid
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	// Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders map[string]string `json:"custom_headers"`

	// Specify a key for event deduplication
	IdempotencyKey string `json:"idempotency_key"`

	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty" swaggerignore:"true"`
}

func (de *DynamicEvent) Validate() error {
	return requiredFieldsError(
		requiredField{len(de.URL) == 0, "url:please provide an endpoint url"},
		requiredField{len(de.Data) == 0, "data:please provide your webhook event data"},
		requiredField{len(de.EventType) == 0, "event_type:please provide an event type"},
	)
}

type SearchParams struct {
	// The start date
	StartDate string `json:"startDate" example:"2006-01-02T15:04:05"`
	// The end date
	EndDate string `json:"endDate" example:"2008-05-02T15:04:05"`
}

type QueryListEvent struct {
	// Matches event id prefix, idempotency key, event type, and source name.
	// A JSON object uses payload containment, same as body. Text plus JSON ANDs both.
	Query string `json:"query"`

	// URL-encoded JSON object matched against the event payload.
	// Combined with query as AND when both are set.
	Body string `json:"body"`

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

	var body json.RawMessage
	if raw := strings.TrimSpace(r.URL.Query().Get("body")); raw != "" {
		body = json.RawMessage(raw)
	}

	return &QueryListEventResponse{
		Filter: &datastore.Filter{
			OwnerID:         r.URL.Query().Get("ownerId"),
			Query:           r.URL.Query().Get("query"),
			Body:            body,
			IdempotencyKey:  r.URL.Query().Get("idempotencyKey"),
			BrokerMessageId: r.URL.Query().Get("brokerMessageId"),
			EndpointIDs:     getEndpointIDs(r),
			SourceIDs:       getSourceIDs(r),
			SearchParams:    searchParams,
			Pageable:        m.GetPageableFromContext(r.Context()),
		},
	}, nil
}

type DynamicEventStub struct {
	ProjectID string `json:"project_id"`
	EventType string `json:"event_type" valid:"required~please provide an event type"`
	// Data is an arbitrary JSON value that gets sent as the body of the webhook to the endpoints
	Data           json.RawMessage   `json:"data" valid:"required~please provide your data" swaggertype:"object"`
	CustomHeaders  map[string]string `json:"custom_headers"`
	IdempotencyKey string            `json:"idempotency_key"`
}

func (ds *DynamicEventStub) Validate() error {
	return requiredFieldsError(
		requiredField{len(ds.EventType) == 0, "event_type:please provide an event type"},
		requiredField{len(ds.Data) == 0, "data:please provide your data"},
	)
}

type BroadcastEvent struct {
	JobID   string `json:"jid" swaggerignore:"true"`
	EventID string `json:"event_id" swaggerignore:"true"`

	// Event Type is used for filtering and debugging e.g invoice.paid
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	ProjectID string `json:"project_id" swaggerignore:"true"`
	SourceID  string `json:"source_id" swaggerignore:"true"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" valid:"required~please provide your data" swaggertype:"object"`

	// Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders map[string]string `json:"custom_headers"`

	// Specify a key for event deduplication
	IdempotencyKey string `json:"idempotency_key"`

	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
}

func (bs *BroadcastEvent) Validate() error {
	return requiredFieldsError(
		requiredField{len(bs.EventType) == 0, "event_type:please provide an event type"},
		requiredField{len(bs.Data) == 0, "data:please provide your data"},
	)
}

type FanoutEvent struct {
	// Used for fanout, sends this event to all endpoints with this OwnerID.
	OwnerID string `json:"owner_id" valid:"required~please provide an owner id"`

	// Event Type is used for filtering and debugging e.g invoice.paid
	EventType string `json:"event_type" valid:"required~please provide an event type"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" valid:"required~please provide your data" swaggertype:"object"`

	// Specifies custom headers you want convoy to add when the event is dispatched to your endpoint
	CustomHeaders map[string]string `json:"custom_headers"`

	// Specify a key for event deduplication
	IdempotencyKey string `json:"idempotency_key"`
}

func (fe *FanoutEvent) Validate() error {
	return requiredFieldsError(
		requiredField{len(fe.OwnerID) == 0, "owner_id:please provide an owner id"},
		requiredField{len(fe.EventType) == 0, "event_type:please provide an event type"},
		requiredField{len(fe.Data) == 0, "data:please provide your data"},
	)
}

type EventResponse struct {
	*datastore.Event
}

// EventQueuedResponse is the 201 body from create, broadcast, fan-out, and
// dynamic ingest. The event id is assigned before the worker persists the
// row. Use UID to retrieve the event or list its deliveries. Get and retry
// a delivery need an event delivery id from that list; this receipt does
// not include one.
type EventQueuedResponse struct {
	// UID is the event id.
	UID string `json:"uid"`

	EventType string `json:"event_type,omitempty"`

	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

func NewEventQueuedResponse(uid, eventType, idempotencyKey string) EventQueuedResponse {
	return EventQueuedResponse{
		UID:            uid,
		EventType:      eventType,
		IdempotencyKey: idempotencyKey,
	}
}

type QueryCountAffectedEvents struct {
	SourceID   string `json:"sourceId"`
	EndpointID string `json:"endpointId"`
	SearchParams
}

type QueryCountAffectedEventsResponse struct {
	*datastore.Filter
}

// CountResponse is the data payload for count endpoints (e.g. countbatchreplayevents).
type CountResponse struct {
	Num int64 `json:"num"`
}

// DeliveryStatusTotalsResponse carries per-status delivery totals for a window.
// A status with no deliveries is absent from Totals rather than present as zero,
// so a client can tell an empty window from a failed request. Source names the
// table that answered, either the daily rollup or a live scan.
type DeliveryStatusTotalsResponse struct {
	Totals map[string]int64 `json:"totals"`
	Source string           `json:"source"`
}

// DeliveryFilterEventTypesResponse is the Event Deliveries type dropdown.
// Catalog is declared names (minus "*", deprecated). Observed is distinct
// event_deliveries.event_type values in the date window that are not already
// declared, including names that are declared but deprecated. Ingest does
// not write catalog rows.
type DeliveryFilterEventTypesResponse struct {
	Catalog  []string `json:"catalog"`
	Observed []string `json:"observed"`
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

// GetSearchParams parses startDate/endDate query params into SearchParams, defaulting
// to the last 7 days when absent. Exported so handlers that need range-scoped reads
// (e.g. the endpoint period failure rate) share one date-handling path.
func GetSearchParams(r *http.Request) (datastore.SearchParams, error) {
	return getSearchParams(r)
}

func getSearchParams(r *http.Request) (datastore.SearchParams, error) {
	var searchParams datastore.SearchParams
	format := "2006-01-02T15:04:05"
	startDate := r.URL.Query().Get("startDate")
	endDate := r.URL.Query().Get("endDate")

	var err error

	var startT time.Time
	if len(startDate) == 0 {
		now := time.Now()
		startT = now.AddDate(0, 0, -7)
		startT = time.Date(startT.Year(), startT.Month(), startT.Day(), 0, 0, 0, 0, startT.Location())
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
		Query:          listSearchQuery(r),
	}

	return searchParams, nil
}

func listSearchQuery(r *http.Request) string {
	q := strings.TrimSpace(r.URL.Query().Get("query"))
	if q == "" {
		q = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	return q
}
