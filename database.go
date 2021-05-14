package hookcamp

import "io"

// Datastore provides an abstraction for all database related operations
type Datastore interface {
	OrganisationRepository
	io.Closer
	Migrate() error
}
