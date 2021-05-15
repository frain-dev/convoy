package datastore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"gorm.io/gorm"
)

type appRepo struct {
	inner *gorm.DB
}

func NewApplicationRepo(inner *gorm.DB) hookcamp.ApplicationRepository {
	return &appRepo{
		inner: inner,
	}
}

func (db *appRepo) CreateApplication(ctx context.Context,
	app *hookcamp.Application) error {
	if app.ID == uuid.Nil {
		app.ID = uuid.New()
	}

	return db.inner.WithContext(ctx).
		Create(app).
		Error
}

func (db *appRepo) LoadApplications(ctx context.Context) ([]hookcamp.Application, error) {
	apps := make([]hookcamp.Application, 0)

	return apps, db.inner.WithContext(ctx).
		Preload("Organisation").
		Find(&apps).
		Error
}

func (db *appRepo) FindApplicationByID(ctx context.Context,
	id uuid.UUID) (*hookcamp.Application, error) {
	app := new(hookcamp.Application)

	err := db.inner.WithContext(ctx).
		Where(&hookcamp.Application{ID: id}).
		First(app).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = hookcamp.ErrApplicationNotFound
	}

	return app, err
}
