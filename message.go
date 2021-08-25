package hookcamp

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/hookcamp/hookcamp/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MessageStatus string

var (
	ErrMessageNotFound = errors.New("message not found")
)

const (
	// UnknownMessageStatus when we don't know the state of a message
	UnknownMessageStatus MessageStatus = "Unknown"
	// ScheduledMessageStatus : when  a message has been scheduled for
	// delivery
	ScheduledMessageStatus  MessageStatus = "Scheduled"
	ProcessingMessageStatus MessageStatus = "Processing"
	FailureMessageStatus    MessageStatus = "Failure"
	SuccessMessageStatus    MessageStatus = "Success"
	RetryMessageStatus      MessageStatus = "Retry"
)

type MessageMetadata struct {
	// NextSendTime denotes the next time a message will be published in
	// case it failed the first time
	NextSendTime int64 `json:"next_send_time" bson:"next_send_time"`

	// NumTrials: number of times we have tried to deliver this message to
	// an application
	NumTrials int64 `json:"num_trials" bson:"num_trials"`

	RetryLimit int64 `json:"retry_limit" bson:"retry_limit"`
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

	Application *Application `json:"application,omitempty" bson:"application"`

	MessageAttempts []MessageAttempt `json:"attempts" bson:"attempts"`

	CreatedAt int64 `json:"created_at" bson:"created_at"`
	UpdatedAt int64 `json:"updated_at" bson:"updated_at"`
	DeletedAt int64 `json:"deleted_at" bson:"deleted_at"`
}

// Message defines a payload to be sent to an application
type MessageAttempt struct {
	ID         primitive.ObjectID `json:"-" bson:"_id"`
	UID        string             `json:"uid" bson:"uid"`
	MsgID      string             `json:"msg_id" bson:"msg_id"`
	EndpointID string             `json:"endpoint_id" bson:"endpoint_id"`
	APIVersion string             `json:"api_version" bson:"api_version"`

	IPAddress        string `json:"ip_address" bson:"ip_address"`
	UserAgent        string `json:"user_agent" bson:"user_agent"`
	HttpResponseCode string `json:"http_status" bson:"http_status"`
	ResponseData     string `json:"response_data" bson:"response_data"`

	Status MessageStatus `json:"status" bson:"status"`

	Message  Message  `json:"-" bson:"msg"`
	Endpoint Endpoint `json:"-" bson:"endpoint"`

	CreatedAt int64 `json:"created_at" bson:"created_at"`
	UpdatedAt int64 `json:"updated_at" bson:"updated_at"`
	DeletedAt int64 `json:"deleted_at" bson:"deleted_at"`
}

type MessageRepository interface {
	CreateMessage(context.Context, *Message) error
	LoadMessages(context.Context, string, models.SearchParams) ([]Message, error)
	LoadMessagesByAppId(context.Context, string) ([]Message, error)
	FindMessageByID(ctx context.Context, id string) (*Message, error)
	LoadMessagesScheduledForPosting(context.Context) ([]Message, error)
	LoadMessagesForPostingRetry(context.Context) ([]Message, error)
	LoadAbandonedMessagesForPostingRetry(context.Context) ([]Message, error)
	UpdateStatusOfMessages(context.Context, []Message, MessageStatus) error
	UpdateMessage(ctx context.Context, m Message) error
	LoadMessagesPaged(context.Context, models.Pageable) ([]Message, pager.PaginationData, error)
}
