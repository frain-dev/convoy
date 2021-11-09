package convoy

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/frain-dev/convoy/server/models"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrEventNotFound = errors.New("event not found")
)

type AppMetadata struct {
	GroupID      string `json:"group_id" bson:"group_id"`
	SupportEmail string `json:"support_email" bson:"support_email"`
}

// EventType is used to identify an specific event.
// This could be "user.new"
// This will be used for data indexing
// Makes it easy to filter by a list of events
type EventType string

//Event defines a payload to be sent to an application
type Event struct {
	ID        primitive.ObjectID `json:"-" bson:"_id"`
	UID       string             `json:"uid" bson:"uid"`
	AppID     string             `json:"app_id" bson:"app_id"`
	EventType EventType          `json:"event_type" bson:"event_type"`

	// ProviderID is a custom ID that can be used to reconcile this Event
	// with your internal systems.
	// This is optional
	// If not provided, we will generate one for you
	ProviderID string `json:"provider_id" bson:"provider_id"`

	// Data is an arbitrary JSON value that gets sent as the body of the
	// webhook to the endpoints
	Data json.RawMessage `json:"data" bson:"data"`

	AppMetadata *AppMetadata `json:"app_metadata,omitempty" bson:"app_metadata"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type EventRepository interface {
	CreateEvent(context.Context, *Event) error
	LoadEventIntervals(context.Context, string, models.SearchParams, Period, int) ([]models.EventInterval, error)
	LoadEventsPagedByAppId(context.Context, string, models.SearchParams, models.Pageable) ([]Event, pager.PaginationData, error)
	FindEventByID(ctx context.Context, id string) (*Event, error)
	LoadEventsScheduledForPosting(context.Context) ([]Event, error)
	LoadEventsForPostingRetry(context.Context) ([]Event, error)
	LoadAbandonedEventsForPostingRetry(context.Context) ([]Event, error)
	LoadEventsPaged(context.Context, string, string, models.SearchParams, models.Pageable) ([]Event, pager.PaginationData, error)
}
