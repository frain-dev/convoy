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

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

func (o *Organisation) IsDeleted() bool { return o.DeletedAt > 0 }

func (o *Organisation) IsOwner(a *Application) bool { return o.UID == a.OrgID }

type OrganisationRepository interface {
	LoadOrganisations(context.Context) ([]*Organisation, error)
	CreateOrganisation(context.Context, *Organisation) error
	UpdateOrganisation(context.Context, *Organisation) error
	FetchOrganisationByID(context.Context, string) (*Organisation, error)
}
