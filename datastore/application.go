package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/hookcamp/hookcamp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type appRepo struct {
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

	app.ID = primitive.NewObjectID()

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
	id string) (*hookcamp.Application, error) {

	app := new(hookcamp.Application)

	filter := bson.D{
		primitive.E{
			Key:   "uid",
			Value: id,
		},
	}

	err := db.client.FindOne(ctx, filter).
		Decode(&app)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = hookcamp.ErrApplicationNotFound
	}

	return app, err
}

func (db *appRepo) UpdateApplication(ctx context.Context,
	app *hookcamp.Application) error {

	app.UpdatedAt = time.Now().Unix()

	filter := bson.D{primitive.E{Key: "uid", Value: app.UID}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "endpoints", Value: app.Endpoints},
		primitive.E{Key: "updated_at", Value: app.UpdatedAt},
		primitive.E{Key: "title", Value: app.Title},
	}}}

	_, err := db.client.UpdateOne(ctx, filter, update)
	return err
}
