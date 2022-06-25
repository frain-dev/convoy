package badger

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type configRepo struct {
	db *badgerhold.Store
}

func NewConfigRepo(db *badgerhold.Store) datastore.ConfigurationRepository {
	return &configRepo{db: db}
}

func (c *configRepo) CreateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	return nil
}

func (c *configRepo) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	return nil, nil
}

func (c *configRepo) UpdateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	return nil
}
