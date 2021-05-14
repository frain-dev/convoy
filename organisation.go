package hookcamp

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organisation is a model that depicts an organisation
type Organisation struct {
	ID   uuid.UUID `json:"id" gorm:"type:uuid;uniqueIndex,not null"`
	Name string    `json:"name" gorm:"not null"`

	gorm.Model
}

// OrganisationRepository provides an abstraction for all organisation
// persistence
type OrganisationRepository interface {
	// LoadOrganisations fetches all known organisations
	LoadOrganisations(context.Context) ([]Organisation, error)

	// CreateOrganisation persists a new org to the database
	CreateOrganisation(context.Context, *Organisation) error
}
