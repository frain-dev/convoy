package hookcamp

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MessageStatus uint

const (
	// UnknownMessageStatus when we don't know the state of a message
	UnknownMessageStatus MessageStatus = iota
	// ScheduledMessageStatus : when  a message has been scheduled for
	// delivery
	ScheduledMessageStatus
	ProcessingMessageStatus
	FailureMessageStatus
	SuccessMessageStatus
	RetryMessageStatus
)

type JSONData json.RawMessage

func (j JSONData) Value() (driver.Value, error) {
	return driver.Value(string(j)), nil
}

type MessageMetadata struct {
	// NextSendTime denotes the next time a message will be published in
	// case it failed the first time
	NextSendTime int64 `json:"next_send_time"`

	// NumTrials: number of times we have tried to deliver this message to
	// an application
	NumTrials int64 `json:"num_trials"`
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
	UID       string             `json:"id" bson:"uid"`
	AppID     string             `json:"app_id" bson:"app_id"`
	EventType EventType          `json:"event_type" bson:"event_type"`

	// ProviderID is a custom ID that can be used to reconcile this message
	// with your internal systems.
	// This is optional
	// If not provided, we will generate one for you
	ProviderID string `json:"provider_id" bson:"provider_id"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data JSONData `json:"data" bson:"data"`

	Metadata *MessageMetadata `json:"metadata" bson:"metadata"`

	Status MessageStatus `json:"status" bson:"status"`

	Application Application `json:"application" bson:"application"`

	CreatedAt int64 `json:"created_at" bson:"created_at"`
	UpdatedAt int64 `json:"updated_at" bson:"updated_at"`
	DeletedAt int64 `json:"deleted_at" bson:"deleted_at"`
}

type MessageRepository interface {
	CreateMessage(context.Context, *Message) error
}
