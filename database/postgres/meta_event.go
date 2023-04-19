package postgres

import (
	"context"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
)

type metaEventRepo struct {
	db *sqlx.DB
}

func NewMetaEventRepo(db database.Database) datastore.MetaEventRepository {
	return &metaEventRepo{db: db.GetDB()}
}

func (m *metaEventRepo) CreateMetaEvent(ctx context.Context, metaEvent *datastore.MetaEvent) error {
	return nil
}

func (m *metaEventRepo) LoadMetaEventsPaged(ctx context.Context, projectID string, f *datastore.Filter) ([]datastore.MetaEvent, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}
