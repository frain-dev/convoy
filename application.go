package hookcamp

import (
	"context"
	"errors"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/hookcamp/hookcamp/server/models"

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

	Endpoints []Endpoint `json:"endpoints" bson:"endpoints"`
	CreatedAt int64      `json:"created_at" bson:"created_at"`
	UpdatedAt int64      `json:"updated_at" bson:"updated_at"`
	DeletedAt int64      `json:"deleted_at,omitempty" bson:"deleted_at"`
}

type Endpoint struct {
	UID         string `json:"uid" bson:"uid"`
	TargetURL   string `json:"target_url" bson:"target_url"`
	Secret      string `json:"secret" bson:"secret"`
	Description string `json:"description" bson:"description"`

	Merged *bool `json:"merged,omitempty" bson:"merged"`

	CreatedAt int64 `json:"created_at" bson:"created_at"`
	UpdatedAt int64 `json:"updated_at" bson:"updated_at"`
	DeletedAt int64 `json:"deleted_at" bson:"deleted_at"`
}

type ApplicationRepository interface {
	CreateApplication(context.Context, *Application) error
	LoadApplications(context.Context) ([]Application, error)
	FindApplicationByID(context.Context, string) (*Application, error)
	UpdateApplication(context.Context, *Application) error
	LoadApplicationsPagedByOrgId(context.Context, string, models.Pageable) ([]Application, pager.PaginationData, error)
	SearchApplicationsByOrgId(context.Context, string, models.SearchParams) ([]Application, error)
}
