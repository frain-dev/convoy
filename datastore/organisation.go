package datastore

import (
	"context"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
)

func (db *database) LoadOrganisations(ctx context.Context) ([]hookcamp.Organisation, error) {

	var orgs = make([]hookcamp.Organisation, 0)

	return orgs, db.inner.WithContext(ctx).
		Find(&orgs).Error
}

func (db *database) CreateOrganisation(ctx context.Context, o *hookcamp.Organisation) error {

	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}

	return db.inner.WithContext(ctx).Create(o).Error
}
