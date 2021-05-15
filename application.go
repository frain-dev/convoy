package hookcamp

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrApplicationNotFound is returned when an application cannot be
// found
var ErrApplicationNotFound = errors.New("application not found")

// Application defines an entity that can receive webhooks.
type Application struct {
	ID    uuid.UUID `json:"id" gorm:"type:varchar(220);uniqueIndex;not null"`
	OrgID uuid.UUID `json:"org_id" gorm:"not null"`
	Title string    `json:"name" gorm:"not null;type:varchar(200)"`

	gorm.Model
	Organisation Organisation `json:"organisation" gorm:"foreignKey:OrgID"`
}

// Endpoint defines a target service that can be reached in an application
type Endpoint struct {
	ID          uuid.UUID `json:"id" gorm:"type:varchar(220);uniqueIndex;not null"`
	AppID       uuid.UUID `json:"app_id" gorm:"size:200;not null"`
	TargetURL   string    `json:"target_url" gorm:"not null"`
	Secret      string    `json:"secret" gorm:"type:varchar(200);uniqueIndex;not null"`
	Description string    `json:"description" gorm:"size:220;default:''"`

	Application Application `json:"-" gorm:"foreignKey:AppID"`
	gorm.Model
}

// ApplicationRepository is an abstraction over all database operations of an
// application
type ApplicationRepository interface {
	// CreateApplication when called persists an application to the database
	CreateApplication(context.Context, *Application) error

	// LoadApplications fetches a list of all apps from the database
	LoadApplications(context.Context) ([]Application, error)

	// FindApplicationByID looks for an application by the provided ID.
	FindApplicationByID(context.Context, uuid.UUID) (*Application, error)
}

// EndpointRepository is an abstraction over all endpoint operations with the
// database
type EndpointRepository interface {
	// CreateEndpoint adds a new endpoint to the database
	CreateEndpoint(context.Context, *Endpoint) error
}
