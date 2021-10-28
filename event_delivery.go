package convoy

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server/models"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EventDeliveryStatus string
type HttpHeader map[string]string

var (
	ErrEventDeliveryAttemptNotFound = errors.New("delivery attempt not found")
)

const (
	// ScheduledEventStatus : when  a Event has been scheduled for delivery
	ScheduledEventStatus  EventDeliveryStatus = "Scheduled"
	ProcessingEventStatus EventDeliveryStatus = "Processing"
	DiscardedEventStatus  EventDeliveryStatus = "Discarded"
	FailureEventStatus    EventDeliveryStatus = "Failure"
	SuccessEventStatus    EventDeliveryStatus = "Success"
	RetryEventStatus      EventDeliveryStatus = "Retry"
)

type EventMetadata struct {
	Strategy config.StrategyProvider `json:"strategy" bson:"strategy"`
	// NextSendTime denotes the next time a Event will be published in
	// case it failed the first time
	NextSendTime primitive.DateTime `json:"next_send_time" bson:"next_send_time"`

	// NumTrials: number of times we have tried to deliver this Event to
	// an application
	NumTrials uint64 `json:"num_trials" bson:"num_trials"`

	IntervalSeconds uint64 `json:"interval_seconds" bson:"interval_seconds"`

	RetryLimit uint64 `json:"retry_limit" bson:"retry_limit"`
}

type EndpointMetadata struct {
	UID       string         `json:"uid" bson:"uid"`
	TargetURL string         `json:"target_url" bson:"target_url"`
	Status    EndpointStatus `json:"status" bson:"status"`
	Secret    string         `json:"secret" bson:"secret"`

	Sent bool `json:"sent" bson:"sent"`
}

func (em EventMetadata) Value() (driver.Value, error) {
	b := new(bytes.Buffer)

	if err := json.NewEncoder(b).Encode(em); err != nil {
		return driver.Value(""), err
	}

	return driver.Value(b.String()), nil
}

type EventAttempt struct {
	ID         primitive.ObjectID `json:"-" bson:"_id"`
	UID        string             `json:"uid" bson:"uid"`
	MsgID      string             `json:"msg_id" bson:"msg_id"`
	URL        string             `json:"url" bson:"url"`
	Method     string             `json:"method" bson:"method"`
	EndpointID string             `json:"endpoint_id" bson:"endpoint_id"`
	APIVersion string             `json:"api_version" bson:"api_version"`

	IPAddress        string     `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	RequestHeader    HttpHeader `json:"request_http_header,omitempty" bson:"request_http_header,omitempty"`
	ResponseHeader   HttpHeader `json:"response_http_header,omitempty" bson:"response_http_header,omitempty"`
	HttpResponseCode string     `json:"http_status,omitempty" bson:"http_status,omitempty"`
	ResponseData     string     `json:"response_data,omitempty" bson:"response_data,omitempty"`
	Error            string     `json:"error,omitempty" bson:"error,omitempty"`
	Status           bool       `json:"status,omitempty" bson:"status,omitempty"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`
}

//Event defines a payload to be sent to an application
type EventDelivery struct {
	ID primitive.ObjectID `json:"-" bson:"_id"`

	// Event to be sent to a specific endpoint.
	Event Event `json:"event", bson:"event"`

	// Endpoint contains the destination of the event.
	Endpoint EndpointMetadata `json:"endpoints" bson:"endpoints"`

	AppMetadata   *AppMetadata        `json:"app_metadata,omitempty" bson:"app_metadata"`
	Metadata      *EventMetadata      `json:"metadata" bson:"metadata"`
	Description   string              `json:"description,omitempty" bson:"description"`
	Status        EventDeliveryStatus `json:"status" bson:"status"`
	EventAttempts []EventAttempt      `json:"-" bson:"attempts"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type EventDeliveryRepository interface {
	CreateEventDelivery(context.Context, *EventDelivery) error
	LoadEventIntervals(context.Context, string, models.SearchParams, Period, int) ([]models.EventInterval, error)
	LoadEventsPagedByAppId(context.Context, string, models.SearchParams, models.Pageable) ([]Event, pager.PaginationData, error)
	FindEventByID(ctx context.Context, id string) (*Event, error)
	LoadEventsScheduledForPosting(context.Context) ([]Event, error)
	LoadEventsForPostingRetry(context.Context) ([]Event, error)
	LoadAbandonedEventsForPostingRetry(context.Context) ([]Event, error)
	UpdateStatusOfEventDeliveries(context.Context, []EventDelivery, EventDeliveryStatus) error
	UpdateEventWithAttempt(ctx context.Context, e Event, attempt EventAttempt) error
	LoadEventsPaged(context.Context, string, string, models.SearchParams, models.Pageable) ([]Event, pager.PaginationData, error)
}
