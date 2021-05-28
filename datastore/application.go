package datastore

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

type appRepo struct {
	inner  *gorm.DB
	client *mongo.Collection
}

const (
	appCollections = "applications"
)

func NewApplicationRepo(client *mongo.Database) hookcamp.ApplicationRepository {
	return &appRepo{
		client: client.Collection(appCollections, nil),
	}
}

func (db *appRepo) CreateApplication(ctx context.Context,
	app *hookcamp.Application) error {
	if app.UID == uuid.Nil {
		app.UID = uuid.New()
	}

	_, err := db.client.InsertOne(ctx, app)
	return err
}

func (db *appRepo) LoadApplications(ctx context.Context) (
	[]hookcamp.Application, error) {

	apps := make([]hookcamp.Application, 0)

	cur, err := db.client.Find(ctx, nil)
	if err != nil {
		return apps, err
	}

	for cur.Next(ctx) {
		var org hookcamp.Application
		if err := cur.Decode(&org); err != nil {
			return apps, err
		}
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return apps, err
	}

	return apps, nil
}

func (db *appRepo) FindApplicationByID(ctx context.Context,
	id uuid.UUID) (*hookcamp.Application, error) {

	app := new(hookcamp.Application)

	err := db.client.FindOne(ctx, bson.M{"uid": id}).
		Decode(&app)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = hookcamp.ErrApplicationNotFound
	}

	return app, err
}
