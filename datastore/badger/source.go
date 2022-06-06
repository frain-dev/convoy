package badger

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type sourceRepo struct {
	db *badgerhold.Store
}

func NewSourceRepo(db *badgerhold.Store) datastore.SourceRepository {
	return &sourceRepo{db: db}
}

func (s *sourceRepo) CreateSource(ctx context.Context, source *datastore.Source) error {
	return nil
}

func (s *sourceRepo) UpdateSource(ctx context.Context, groupId string, source *datastore.Source) error {
	return nil
}

func (s *sourceRepo) FindSourceByID(ctx context.Context, groupId string, id string) (*datastore.Source, error) {
	return nil, nil
}

func (s *sourceRepo) FindSourceByMaskID(ctx context.Context, maskId string) (*datastore.Source, error) {
	return nil, nil
}

func (s *sourceRepo) DeleteSourceByID(ctx context.Context, groupId string, id string) error {
	return nil
}

func (s *sourceRepo) LoadSourcesPaged(ctx context.Context, groupId string, f *datastore.SourceFilter, pageable datastore.Pageable) ([]datastore.Source, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}
