package hookcamp

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrOrganisationNotFound is an error that should be thrown when an
// organisation cannot be retrieved from the datastore
var ErrOrganisationNotFound = errors.New("organisation not found")

// Organisation is a model that depicts an organisation
type Organisation struct {
	UID     uuid.UUID `json:"uid"`
	OrgName string    `json:"name"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
	DeletedAt int64 `json:"deleted_at"`
}

func (o Organisation) IsDeleted() bool { return o.DeletedAt > 0 }

type OrganisationRepository interface {
	LoadOrganisations(context.Context) ([]Organisation, error)
	CreateOrganisation(context.Context, *Organisation) error
	FetchOrganisationByID(context.Context, uuid.UUID) (*Organisation, error)
}
