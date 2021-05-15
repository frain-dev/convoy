package datastore

import (
	"context"

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

	return db.inner.WithContext(ctx).Create(app).Error
}

func (db *appRepo) LoadApplications(ctx context.Context) ([]hookcamp.Application, error) {

	var apps = make([]hookcamp.Application, 0)

	return apps, db.inner.WithContext(ctx).
		Preload("Organisation").
		Find(&apps).Error
}
