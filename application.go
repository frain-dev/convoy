package hookcamp

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	// ErrApplicationNotFound is returned when an application cannot be
	// found
	ErrApplicationNotFound = errors.New("application not found")

	// ErrEndpointNotFound is returned when an endpoint cannot be found
	ErrEndpointNotFound = errors.New("endpoint not found")
)

type Application struct {
	UID   uuid.UUID `json:"uid"`
	OrgID uuid.UUID `json:"org_id"`
	Title string    `json:"name"`

	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint defines a target service that can be reached in an application
type Endpoint struct {
	ID          uuid.UUID `json:"id"`
	AppID       uuid.UUID `json:"app_id"`
	TargetURL   string    `json:"target_url"`
	Secret      string    `json:"secret"`
	Description string    `json:"description"`

	Application Application `json:"-"`
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

	// FindEndpointByID retrieves an endpoint by the proovided ID
	FindEndpointByID(context.Context, uuid.UUID) (*Endpoint, error)
}
