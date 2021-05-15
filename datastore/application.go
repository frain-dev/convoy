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
