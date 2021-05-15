package hookcamp

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// ErrOrganisationNotFound is an error that should be thrown when an
	// organisation cannot be retrieved from the datastore
	ErrOrganisationNotFound = errors.New("organisation not found")
)

// Organisation is a model that depicts an organisation
type Organisation struct {
	ID      uuid.UUID `json:"id" gorm:"type:uuid;uniqueIndex;not null"`
	OrgName string    `json:"name" gorm:"not null"`

	gorm.Model
}

// OrganisationRepository provides an abstraction for all organisation
// persistence
type OrganisationRepository interface {
	// LoadOrganisations fetches all known organisations
	LoadOrganisations(context.Context) ([]Organisation, error)

	// CreateOrganisation persists a new org to the database
	CreateOrganisation(context.Context, *Organisation) error

	// FetchOrganisationByID retrieves a given organisation by the provided
	// uuid
	FetchOrganisationByID(context.Context, uuid.UUID) (*Organisation, error)
}
