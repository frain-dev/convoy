package hookcamp

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrOrganisationNotFound = errors.New("organisation not found")

type Organisation struct {
	ID      primitive.ObjectID `json:"-" bson:"_id"`
	UID     string             `json:"uid" bson:"uid"`
	OrgName string             `json:"name" bson:"org_name"`
	ApiKey  string             `json:"api_key" bson:"api_key"`

	CreatedAt int64 `json:"created_at" bson:"created_at"`
	UpdatedAt int64 `json:"updated_at" bson:"updated_at"`
	DeletedAt int64 `json:"deleted_at" bson:"deleted_at"`
}

func (o Organisation) IsDeleted() bool { return o.DeletedAt > 0 }

type OrganisationRepository interface {
	LoadOrganisations(context.Context) ([]Organisation, error)
	CreateOrganisation(context.Context, *Organisation) error
	FetchOrganisationByID(context.Context, string) (*Organisation, error)
}
