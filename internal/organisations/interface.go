package organisations

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
)

// OrganisationRepository defines the interface for organisation operations using SQLc
type OrganisationRepository interface {
	// CreateOrganisation creates a new organisation
	CreateOrganisation(ctx context.Context, org *datastore.Organisation) error

	// UpdateOrganisation updates an existing organisation
	UpdateOrganisation(ctx context.Context, org *datastore.Organisation) error

	// DeleteOrganisation soft deletes an organisation by ID
	DeleteOrganisation(ctx context.Context, id string) error

	// FetchOrganisationByID retrieves an organisation by its ID
	FetchOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error)

	// FetchOrganisationByCustomDomain retrieves an organisation by its custom domain
	FetchOrganisationByCustomDomain(ctx context.Context, domain string) (*datastore.Organisation, error)

	// FetchOrganisationByAssignedDomain retrieves an organisation by its assigned domain
	FetchOrganisationByAssignedDomain(ctx context.Context, domain string) (*datastore.Organisation, error)

	// LoadOrganisationsPaged retrieves organisations with pagination
	LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error)

	// LoadOrganisationsPagedWithSearch retrieves organisations with pagination and search
	LoadOrganisationsPagedWithSearch(ctx context.Context, pageable datastore.Pageable, search string) ([]datastore.Organisation, datastore.PaginationData, error)

	// CountOrganisations returns the total count of organisations
	CountOrganisations(ctx context.Context) (int64, error)

	// CalculateUsage calculates usage metrics for an organisation
	CalculateUsage(ctx context.Context, orgID string, startTime, endTime time.Time) (*datastore.OrganisationUsage, error)
}
