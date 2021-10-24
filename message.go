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

type MessageStatus string
type HttpHeader map[string]string

var (
	ErrMessageNotFound = errors.New("event not found")

	ErrMessageDeliveryAttemptNotFound = errors.New("delivery attempt not found")
)

const (
	// ScheduledMessageStatus : when  a message has been scheduled for delivery
	ScheduledMessageStatus  MessageStatus = "Scheduled"
	ProcessingMessageStatus MessageStatus = "Processing"
	DiscardedMessageStatus  MessageStatus = "Discarded"
	FailureMessageStatus    MessageStatus = "Failure"
	SuccessMessageStatus    MessageStatus = "Success"
	RetryMessageStatus      MessageStatus = "Retry"
)

type MessageMetadata struct {
	Strategy config.StrategyProvider `json:"strategy" bson:"strategy"`
	// NextSendTime denotes the next time a message will be published in
	// case it failed the first time
	NextSendTime primitive.DateTime `json:"next_send_time" bson:"next_send_time"`

	// NumTrials: number of times we have tried to deliver this message to
	// an application
	NumTrials uint64 `json:"num_trials" bson:"num_trials"`

	IntervalSeconds uint64 `json:"interval_seconds" bson:"interval_seconds"`

	RetryLimit uint64 `json:"retry_limit" bson:"retry_limit"`
}

type AppMetadata struct {
	GroupID      string `json:"group_id" bson:"group_id"`
	Secret       string `json:"secret" bson:"secret"`
	SupportEmail string `json:"support_email" bson:"support_email"`

	Endpoints []EndpointMetadata `json:"endpoints" bson:"endpoints"`
}

type EndpointMetadata struct {
	UID       string         `json:"uid" bson:"uid"`
	TargetURL string         `json:"target_url" bson:"target_url"`
	Status    EndpointStatus `json:"status" bson:"status"`

	Sent bool `json:"sent" bson:"sent"`
}

func (m MessageMetadata) Value() (driver.Value, error) {
	b := new(bytes.Buffer)

	if err := json.NewEncoder(b).Encode(m); err != nil {
		return driver.Value(""), err
	}

	return driver.Value(b.String()), nil
}

// EventType is used to identify an specific event.
// This could be "user.new"
// This will be used for data indexing
// Makes it easy to filter by a list of events
type EventType string

// Message defines a payload to be sent to an application
type Message struct {
	ID        primitive.ObjectID `json:"-" bson:"_id"`
	UID       string             `json:"uid" bson:"uid"`
	AppID     string             `json:"app_id" bson:"app_id"`
	EventType EventType          `json:"event_type" bson:"event_type"`

	// ProviderID is a custom ID that can be used to reconcile this message
	// with your internal systems.
	// This is optional
	// If not provided, we will generate one for you
	ProviderID string `json:"provider_id" bson:"provider_id"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" bson:"data"`

	Metadata *MessageMetadata `json:"metadata" bson:"metadata"`

	Description string `json:"description,omitempty" bson:"description"`

	Status MessageStatus `json:"status" bson:"status"`

	AppMetadata *AppMetadata `json:"app_metadata,omitempty" bson:"app_metadata"`

	MessageAttempts []MessageAttempt `json:"-" bson:"attempts"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type MessageAttempt struct {
	ID         primitive.ObjectID `json:"-" bson:"_id"`
	UID        string             `json:"uid" bson:"uid"`
	MsgID      string             `json:"msg_id" bson:"msg_id"`
	URL        string             `json:"url" bson:"url"`
	Method     string             `json:"method" bson:"method"`
	EndpointID string             `json:"endpoint_id" bson:"endpoint_id"`
	APIVersion string             `json:"api_version" bson:"api_version"`

	IPAddress        string        `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	RequestHeader    HttpHeader    `json:"request_http_header,omitempty" bson:"request_http_header,omitempty"`
	ResponseHeader   HttpHeader    `json:"response_http_header,omitempty" bson:"response_http_header,omitempty"`
	HttpResponseCode string        `json:"http_status,omitempty" bson:"http_status,omitempty"`
	ResponseData     string        `json:"response_data,omitempty" bson:"response_data,omitempty"`
	Error            string        `json:"error,omitempty" bson:"error,omitempty"`
	Status           MessageStatus `json:"status,omitempty" bson:"status,omitempty"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`
}

type MessageRepository interface {
	CreateMessage(context.Context, *Message) error
	LoadMessageIntervals(context.Context, string, models.SearchParams, Period, int) ([]models.MessageInterval, error)
	LoadMessagesPagedByAppId(context.Context, string, models.SearchParams, models.Pageable) ([]Message, pager.PaginationData, error)
	FindMessageByID(ctx context.Context, id string) (*Message, error)
	LoadMessagesScheduledForPosting(context.Context) ([]Message, error)
	LoadMessagesForPostingRetry(context.Context) ([]Message, error)
	LoadAbandonedMessagesForPostingRetry(context.Context) ([]Message, error)
	UpdateStatusOfMessages(context.Context, []Message, MessageStatus) error
	UpdateMessageWithAttempt(ctx context.Context, m Message, attempt MessageAttempt) error
	LoadMessagesPaged(context.Context, string, string, models.SearchParams, models.Pageable) ([]Message, pager.PaginationData, error)
}
