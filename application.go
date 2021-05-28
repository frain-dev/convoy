package hookcamp

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrApplicationNotFound = errors.New("application not found")

	ErrEndpointNotFound = errors.New("endpoint not found")
)

type Application struct {
	UID   uuid.UUID `json:"uid"`
	OrgID uuid.UUID `json:"org_id"`
	Title string    `json:"name"`

	Endpoints []Endpoint `json:"endpoints"`
}

type Endpoint struct {
	UID         uuid.UUID `json:"uid"`
	AppID       uuid.UUID `json:"app_id"`
	TargetURL   string    `json:"target_url"`
	Secret      string    `json:"secret"`
	Description string    `json:"description"`

	Application Application `json:"-"`
}

type ApplicationRepository interface {
	CreateApplication(context.Context, *Application) error
	LoadApplications(context.Context) ([]Application, error)
	FindApplicationByID(context.Context, uuid.UUID) (*Application, error)
}

type EndpointRepository interface {
	CreateEndpoint(context.Context, *Endpoint) error
	FindEndpointByID(context.Context, uuid.UUID) (*Endpoint, error)
}
