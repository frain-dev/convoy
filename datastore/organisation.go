package datastore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"gorm.io/gorm"
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

func (db *database) FetchOrganisationByID(ctx context.Context, id uuid.UUID) (*hookcamp.Organisation, error) {

	var org = new(hookcamp.Organisation)

	err := db.inner.WithContext(ctx).
		Where(&hookcamp.Organisation{ID: id}).
		First(org).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = hookcamp.ErrOrganisationNotFound
	}

	return org, err
}
