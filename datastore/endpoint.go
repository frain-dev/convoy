package datastore

import (
	"context"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"gorm.io/gorm"
)

type endpointDB struct {
	inner *gorm.DB
}

func NewEndpointRepoository(db *gorm.DB) hookcamp.EndpointRepository {
	return &endpointDB{
		inner: db,
	}
}

func (e *endpointDB) CreateEndpoint(ctx context.Context,
	endpoint *hookcamp.Endpoint) error {
	if endpoint.ID == uuid.Nil {
		endpoint.ID = uuid.New()
	}

	return e.inner.WithContext(ctx).
		Create(endpoint).
		Error
}
