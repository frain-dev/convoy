package hookcamp

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageStatus is the current state of a message
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

// MessageMetada stores more information about a specific message
type MessageMetada struct {
	// NextSendTime denotes the next time a message will be published in
	// case it failed the first time
	NextSendTime int64 `json:"next_send_time"`

	// NumTrials: number of times we have tried to deliver this message to
	// an application
	NumTrials int64 `json:"num_trials"`
}

// EventType is used to identify an specific event.
// This could be "user.new"
type EventType string

// Message defines a payload to be sent to an application
type Message struct {
	ID        uuid.UUID `json:"id" gorm:"type:varchar(220);uniqueIndex;not null"`
	AppID     uuid.UUID `json:"app_id" gorm:"size:200;not null"`
	EventType EventType `json:"event_type" gorm:"type:varchar(220);index;not null"`

	// ProviderID is a custom ID that can be used to reconcile this message
	// with your internal systems.
	// This is optional
	ProviderID string `json:"provider_id" gorm:"type:varchar(220);index"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" gorm:"type:text;not null"`

	Metadata MessageMetada `json:"metadata" gorm:"type:text;not null"`

	Status MessageStatus `json:"status" gorm:"type:smallint; not null; default:1"`

	gorm.Model
	Application Application `json:"application" gorm:"foreignKey:AppID"`
}

// MessageRepository is an abstraction over database operations of a message
type MessageRepository interface {
	// CreateMessage will persist a message to the database
	CreateMessage(context.Context, *Message) error
}
