package hookcamp

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organisation is a model that depicts an organisation
type Organisation struct {
	ID   uuid.UUID `json:"id" gorm:"uniqueIndex,not null"`
	Name string    `json:"name"`

	gorm.Model
}

// OrganisationRepository provides an abstraction for all organisation
// persistence
type OrganisationRepository interface {
	// LoadOrganisations fetches all known organisations
	LoadOrganisations() ([]Organisation, error)
}
