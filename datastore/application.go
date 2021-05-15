package datastore

import (
	"context"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
)

func (db *database) CreateApplication(ctx context.Context,
	app *hookcamp.Application) error {

	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}

	return db.inner.WithContext(ctx).Create(app).Error
}

func (db *database) LoadApplications(ctx context.Context) ([]hookcamp.Application, error) {

	var apps = make([]hookcamp.Application, 0)

	return apps, db.inner.WithContext(ctx).
		Preload("Organisation").
		Find(&apps).Error
}
