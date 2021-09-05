package convoy

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/server/models"
	pager "github.com/gobeam/mongo-go-pagination"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrApplicationNotFound = errors.New("application not found")

	ErrEndpointNotFound = errors.New("endpoint not found")
)

type Application struct {
	ID    primitive.ObjectID `json:"-" bson:"_id"`
	UID   string             `json:"uid" bson:"uid"`
	OrgID string             `json:"org_id" bson:"org_id"`
	Title string             `json:"name" bson:"title"`

	Secret string `json:"secret" bson:"secret"`

	Endpoints []Endpoint         `json:"endpoints" bson:"endpoints"`
	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type Endpoint struct {
	UID         string `json:"uid" bson:"uid"`
	TargetURL   string `json:"target_url" bson:"target_url"`
	Description string `json:"description" bson:"description"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type ApplicationRepository interface {
	CreateApplication(context.Context, *Application) error
	LoadApplications(context.Context, string) ([]Application, error)
	FindApplicationByID(context.Context, string) (*Application, error)
	UpdateApplication(context.Context, *Application) error
	DeleteApplication(context.Context, *Application) error
	LoadApplicationsPagedByOrgId(context.Context, string, models.Pageable) ([]Application, pager.PaginationData, error)
	SearchApplicationsByOrgId(context.Context, string, models.SearchParams) ([]Application, error)
}
