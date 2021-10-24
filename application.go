package convoy

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/server/models"
	pager "github.com/gobeam/mongo-go-pagination"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EndpointStatus string

var (
	ErrApplicationNotFound = errors.New("application not found")

	ErrEndpointNotFound = errors.New("endpoint not found")
)

const (
	ActiveEndpointStatus   EndpointStatus = "active"
	InactiveEndpointStatus EndpointStatus = "inactive"
	PendingEndpointStatus  EndpointStatus = "pending"
)

type Application struct {
	ID           primitive.ObjectID `json:"-" bson:"_id"`
	UID          string             `json:"uid" bson:"uid"`
	GroupID      string             `json:"group_id" bson:"group_id"`
	Title        string             `json:"name" bson:"title"`
	SupportEmail string             `json:"support_email" bson:"support_email"`

	Secret string `json:"secret" bson:"secret"`

	Endpoints []Endpoint         `json:"endpoints" bson:"endpoints"`
	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`

	Events int64 `json:"events" bson:"-"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type Endpoint struct {
	UID         string         `json:"uid" bson:"uid"`
	TargetURL   string         `json:"target_url" bson:"target_url"`
	Description string         `json:"description" bson:"description"`
	Status      EndpointStatus `json:"status" bson:"status"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type ApplicationRepository interface {
	CreateApplication(context.Context, *Application) error
	LoadApplicationsPaged(context.Context, string, models.Pageable) ([]Application, pager.PaginationData, error)
	FindApplicationByID(context.Context, string) (*Application, error)
	UpdateApplication(context.Context, *Application) error
	DeleteApplication(context.Context, *Application) error
	LoadApplicationsPagedByGroupId(context.Context, string, models.Pageable) ([]Application, pager.PaginationData, error)
	SearchApplicationsByGroupId(context.Context, string, models.SearchParams) ([]Application, error)
	FindApplicationEndpointByID(context.Context, string, string) (*Endpoint, error)
	UpdateApplicationEndpointsStatus(context.Context, string, []string, EndpointStatus) error
}
