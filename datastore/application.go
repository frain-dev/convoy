package datastore

import (
	"context"
	"errors"
	"github.com/hookcamp/hookcamp/server/models"
	"time"

	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/hookcamp/hookcamp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type appRepo struct {
	client *mongo.Collection
}

const (
	AppCollections = "applications"
)

func NewApplicationRepo(client *mongo.Database) hookcamp.ApplicationRepository {
	return &appRepo{
		client: client.Collection(AppCollections, nil),
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

	cur, err := db.client.Find(ctx, bson.D{{}})
	if err != nil {
		return apps, err
	}

	for cur.Next(ctx) {
		var app hookcamp.Application
		if err := cur.Decode(&app); err != nil {
			return apps, err
		}

		apps = append(apps, app)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return apps, err
	}

	return apps, nil
}

func (db *appRepo) LoadApplicationsPagedByOrgId(ctx context.Context, orgId string, pageable models.Pageable) ([]hookcamp.Application, pager.PaginationData, error) {

	filter := bson.D{
		primitive.E{
			Key:   "org_id",
			Value: orgId,
		},
	}

	var applications []hookcamp.Application
	paginatedData, err := pager.New(db.client).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", -1).Filter(filter).Decode(&applications).Find()
	if err != nil {
		return applications, pager.PaginationData{}, err
	}

	if applications == nil {
		applications = make([]hookcamp.Application, 0)
	}

	return applications, paginatedData.Pagination, nil
}

func (db *appRepo) SearchApplicationsByOrgId(ctx context.Context, orgId string, searchParams models.SearchParams) ([]hookcamp.Application, error) {

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = searchParams.CreatedAtStart
	}

	filter := bson.M{"org_id": orgId, "created_at": bson.M{"$gte": primitive.NewDateTimeFromTime(time.Unix(start, 0)), "$lte": primitive.NewDateTimeFromTime(time.Unix(end, 0))}}

	apps := make([]hookcamp.Application, 0)
	cur, err := db.client.Find(ctx, filter)
	if err != nil {
		return apps, err
	}

	for cur.Next(ctx) {
		var app hookcamp.Application
		if err := cur.Decode(&app); err != nil {
			return apps, err
		}

		apps = append(apps, app)
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

	app.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.D{primitive.E{Key: "uid", Value: app.UID}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "endpoints", Value: app.Endpoints},
		primitive.E{Key: "updated_at", Value: app.UpdatedAt},
		primitive.E{Key: "title", Value: app.Title},
	}}}

	_, err := db.client.UpdateOne(ctx, filter, update)
	return err
}
