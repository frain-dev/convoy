package datastore

import (
	"context"
	"errors"
	"github.com/hookcamp/hookcamp/server/models"
	"github.com/hookcamp/hookcamp/util"
	log "github.com/sirupsen/logrus"
	"time"

	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/hookcamp/hookcamp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type appRepo struct {
	innerDB *mongo.Database
	client  *mongo.Collection
}

const (
	AppCollections = "applications"
)

func NewApplicationRepo(client *mongo.Database) hookcamp.ApplicationRepository {
	return &appRepo{
		innerDB: client,
		client:  client.Collection(AppCollections, nil),
	}
}

func (db *appRepo) CreateApplication(ctx context.Context,
	app *hookcamp.Application) error {

	app.ID = primitive.NewObjectID()

	_, err := db.client.InsertOne(ctx, app)
	return err
}

func (db *appRepo) LoadApplications(ctx context.Context, orgId string) ([]hookcamp.Application, error) {

	apps := make([]hookcamp.Application, 0)

	filter := bson.M{"document_status": bson.M{"$ne": hookcamp.DeletedDocumentStatus}}
	if !util.IsStringEmpty(orgId) {
		filter = bson.M{"org_id": orgId, "document_status": bson.M{"$ne": hookcamp.DeletedDocumentStatus}}
	}

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

func (db *appRepo) LoadApplicationsPagedByOrgId(ctx context.Context, orgId string, pageable models.Pageable) ([]hookcamp.Application, pager.PaginationData, error) {

	filter := bson.M{"org_id": orgId, "document_status": bson.M{"$ne": hookcamp.DeletedDocumentStatus}}

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

	filter := bson.M{"org_id": orgId, "document_status": bson.M{"$ne": hookcamp.DeletedDocumentStatus}, "created_at": bson.M{"$gte": primitive.NewDateTimeFromTime(time.Unix(start, 0)), "$lte": primitive.NewDateTimeFromTime(time.Unix(end, 0))}}

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

	filter := bson.M{"uid": id, "document_status": bson.M{"$ne": hookcamp.DeletedDocumentStatus}}

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

	filter := bson.M{"uid": app.UID, "document_status": bson.M{"$ne": hookcamp.DeletedDocumentStatus}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "endpoints", Value: app.Endpoints},
		primitive.E{Key: "updated_at", Value: app.UpdatedAt},
		primitive.E{Key: "title", Value: app.Title},
	}}}

	_, err := db.client.UpdateOne(ctx, filter, update)
	return err
}

func (db *appRepo) DeleteApplication(ctx context.Context,
	app *hookcamp.Application) error {

	updateAsDeleted := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "deleted_at", Value: primitive.NewDateTimeFromTime(time.Now())},
		primitive.E{Key: "document_status", Value: hookcamp.DeletedDocumentStatus},
	}}}

	err := db.updateMessagesInApp(ctx, app, updateAsDeleted)
	if err != nil {
		return err
	}

	err = db.deleteApp(ctx, app, updateAsDeleted)
	if err != nil {
		log.Errorf("%s an error has occurred while deleting app - %s", app.UID, err)

		rollback := bson.D{primitive.E{Key: "$set", Value: bson.D{
			primitive.E{Key: "deleted_at", Value: nil},
			primitive.E{Key: "document_status", Value: hookcamp.ActiveDocumentStatus},
		}}}
		err2 := db.updateMessagesInApp(ctx, app, rollback)
		if err2 != nil {
			log.Errorf("%s failed to rollback deleted app messages - %s", app.UID, err2)
		}

		return err
	}
	return nil
}

func (db *appRepo) updateMessagesInApp(ctx context.Context, app *hookcamp.Application, update bson.D) error {
	var msgOperations []mongo.WriteModel

	updateMessagesOperation := mongo.NewUpdateManyModel()
	msgFilter := bson.M{"app_id": app.UID}
	updateMessagesOperation.SetFilter(msgFilter)
	updateMessagesOperation.SetUpdate(update)
	msgOperations = append(msgOperations, updateMessagesOperation)

	msgCollection := db.innerDB.Collection(MsgCollection)
	res, err := msgCollection.BulkWrite(ctx, msgOperations)
	if err != nil {
		log.Errorf("failed to delete messages in %s. Reason: %s", app.UID, err)
		return err
	}
	log.Infof("results of app messages op: %+v", res)
	return nil
}

func (db *appRepo) deleteApp(ctx context.Context, app *hookcamp.Application, update bson.D) error {
	var appOperations []mongo.WriteModel
	updateAppOperation := mongo.NewUpdateOneModel()
	filter := bson.D{primitive.E{Key: "uid", Value: app.UID}}
	updateAppOperation.SetFilter(filter)
	updateAppOperation.SetUpdate(update)
	appOperations = append(appOperations, updateAppOperation)

	res, err := db.client.BulkWrite(ctx, appOperations)
	if err != nil {
		log.Errorf("failed to delete app %s. Reason: %s", app.UID, err)
		return err
	}
	log.Infof("results of app op: %+v", res)
	return nil
}
