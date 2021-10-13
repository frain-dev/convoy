package convoy

import "io"

// Datastore provides an abstraction for all database related operations
type Datastore interface {
	GroupRepository
	ApplicationRepository
	// EndpointRepository
	io.Closer
	Migrate() error
}
