package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	log "github.com/sirupsen/logrus"

	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type appRepo struct {
	innerDB *mongo.Database
	client  *mongo.Collection
}

func NewApplicationRepo(db *mongo.Database) datastore.ApplicationRepository {
	return &appRepo{
		innerDB: db,
		client:  db.Collection(AppCollection, nil),
	}
}

func (db *appRepo) CreateApplication(ctx context.Context, app *datastore.Application, groupID string) error {
	err := db.assertUniqueAppTitle(ctx, app, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateAppName) {
			return err
		}

		return fmt.Errorf("failed to check if application name is unique: %v", err)
	}

	app.ID = primitive.NewObjectID()
	_, err = db.client.InsertOne(ctx, app)
	return err
}

func (db *appRepo) LoadApplicationsPaged(ctx context.Context, groupID, q string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {

	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}
	if !util.IsStringEmpty(groupID) {
		filter = bson.M{"group_id": groupID, "document_status": datastore.ActiveDocumentStatus}
	}

	if !util.IsStringEmpty(q) {
		filter = bson.M{"group_id": groupID, "document_status": datastore.ActiveDocumentStatus, "title": bson.M{"$regex": primitive.Regex{Pattern: q, Options: "i"}}}
	}

	var apps []datastore.Application
	paginatedData, err := pager.New(db.client).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", -1).Filter(filter).Decode(&apps).Find()
	if err != nil {
		return apps, datastore.PaginationData{}, err
	}

	if apps == nil {
		apps = make([]datastore.Application, 0)
	}

	msgCollection := db.innerDB.Collection(EventCollection)
	for i, app := range apps {
		filter = bson.M{"app_id": app.UID, "document_status": datastore.ActiveDocumentStatus}
		count, err := msgCollection.CountDocuments(ctx, filter)
		if err != nil {
			log.Errorf("failed to count events in %s. Reason: %s", app.UID, err)
			return apps, datastore.PaginationData{}, err
		}
		apps[i].Events = count
	}

	return apps, datastore.PaginationData(paginatedData.Pagination), nil
}

func (db *appRepo) assertUniqueAppTitle(ctx context.Context, app *datastore.Application, groupID string) error {
	f := bson.M{
		"uid":             bson.M{"$ne": app.UID},
		"title":           app.Title,
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	count, err := db.client.CountDocuments(ctx, f)
	if err != nil {
		return err
	}

	if count != 0 {
		return datastore.ErrDuplicateAppName
	}

	return nil
}

func (db *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, groupID string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {

	filter := bson.M{
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	var applications []datastore.Application
	paginatedData, err := pager.New(db.client).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", -1).Filter(filter).Decode(&applications).Find()
	if err != nil {
		return applications, datastore.PaginationData{}, err
	}

	if applications == nil {
		applications = make([]datastore.Application, 0)
	}

	msgCollection := db.innerDB.Collection(EventCollection)
	for i, app := range applications {
		filter = bson.M{"app_id": app.UID, "document_status": datastore.ActiveDocumentStatus}
		count, err := msgCollection.CountDocuments(ctx, filter)
		if err != nil {
			log.Errorf("failed to count events in %s. Reason: %s", app.UID, err)
			return applications, datastore.PaginationData{}, err
		}
		applications[i].Events = count
	}

	return applications, datastore.PaginationData(paginatedData.Pagination), nil
}

func (db *appRepo) CountGroupApplications(ctx context.Context, groupID string) (int64, error) {
	filter := bson.M{
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	count, err := db.client.CountDocuments(ctx, filter)
	if err != nil {
		log.WithError(err).Errorf("failed to count apps in group %s", groupID)
		return 0, err
	}
	return count, nil
}

func (db *appRepo) SearchApplicationsByGroupId(ctx context.Context, groupId string, searchParams datastore.SearchParams) ([]datastore.Application, error) {

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = searchParams.CreatedAtStart
	}

	filter := bson.M{
		"group_id":        groupId,
		"document_status": datastore.ActiveDocumentStatus,
		"created_at": bson.M{
			"$gte": primitive.NewDateTimeFromTime(time.Unix(start, 0)),
			"$lte": primitive.NewDateTimeFromTime(time.Unix(end, 0)),
		},
	}

	apps := make([]datastore.Application, 0)
	cur, err := db.client.Find(ctx, filter)
	if err != nil {
		return apps, err
	}

	for cur.Next(ctx) {
		var app datastore.Application
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

	msgCollection := db.innerDB.Collection(EventCollection)
	for i, app := range apps {
		filter = bson.M{"app_id": app.UID, "document_status": datastore.ActiveDocumentStatus}
		count, err := msgCollection.CountDocuments(ctx, filter)
		if err != nil {
			log.Errorf("failed to count events in %s. Reason: %s", app.UID, err)
			return apps, err
		}
		apps[i].Events = count
	}

	return apps, nil
}

func (db *appRepo) FindApplicationByID(ctx context.Context,
	id string) (*datastore.Application, error) {

	app := new(datastore.Application)

	filter := bson.M{"uid": id, "document_status": datastore.ActiveDocumentStatus}

	err := db.client.FindOne(ctx, filter).
		Decode(&app)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrApplicationNotFound
		return app, err
	}

	msgCollection := db.innerDB.Collection(EventCollection)
	filter = bson.M{"app_id": app.UID, "document_status": datastore.ActiveDocumentStatus}
	count, err := msgCollection.CountDocuments(ctx, filter)
	if err != nil {
		log.Errorf("failed to count events in %s. Reason: %s", app.UID, err)
		return app, err
	}
	app.Events = count

	return app, err
}

func (db *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*datastore.Endpoint, error) {

	app, err := db.FindApplicationByID(context.Background(), appID)
	if err != nil {
		return nil, err
	}

	return findEndpoint(&app.Endpoints, endpointID)
}

func findEndpoint(endpoints *[]datastore.Endpoint, id string) (*datastore.Endpoint, error) {
	for _, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			return &endpoint, nil
		}
	}
	return nil, datastore.ErrEndpointNotFound
}

func (db *appRepo) UpdateApplication(ctx context.Context, app *datastore.Application, groupID string) error {
	err := db.assertUniqueAppTitle(ctx, app, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateAppName) {
			return err
		}

		return fmt.Errorf("failed to check if application name is unique: %v", err)
	}

	app.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.M{"uid": app.UID, "document_status": datastore.ActiveDocumentStatus}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "endpoints", Value: app.Endpoints},
		primitive.E{Key: "updated_at", Value: app.UpdatedAt},
		primitive.E{Key: "title", Value: app.Title},
		primitive.E{Key: "support_email", Value: app.SupportEmail},
		primitive.E{Key: "is_disabled", Value: app.IsDisabled},
	}}}

	_, err = db.client.UpdateOne(ctx, filter, update)
	return err
}

func (db *appRepo) DeleteGroupApps(ctx context.Context, groupID string) error {

	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	_, err := db.client.UpdateMany(ctx, bson.M{"group_id": groupID}, update)
	if err != nil {
		return err
	}

	return nil
}

func (db *appRepo) DeleteApplication(ctx context.Context,
	app *datastore.Application) error {

	updateAsDeleted := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "deleted_at", Value: primitive.NewDateTimeFromTime(time.Now())},
		primitive.E{Key: "document_status", Value: datastore.DeletedDocumentStatus},
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
			primitive.E{Key: "document_status", Value: datastore.ActiveDocumentStatus},
		}}}
		err2 := db.updateMessagesInApp(ctx, app, rollback)
		if err2 != nil {
			log.Errorf("%s failed to rollback deleted app messages - %s", app.UID, err2)
		}

		return err
	}
	return nil
}

func (db *appRepo) updateMessagesInApp(ctx context.Context, app *datastore.Application, update bson.D) error {
	var msgOperations []mongo.WriteModel

	updateMessagesOperation := mongo.NewUpdateManyModel()
	msgFilter := bson.M{"app_id": app.UID}
	updateMessagesOperation.SetFilter(msgFilter)
	updateMessagesOperation.SetUpdate(update)
	msgOperations = append(msgOperations, updateMessagesOperation)

	msgCollection := db.innerDB.Collection(EventCollection)
	res, err := msgCollection.BulkWrite(ctx, msgOperations)
	if err != nil {
		log.Errorf("failed to delete messages in %s. Reason: %s", app.UID, err)
		return err
	}
	log.Infof("results of app messages op: %+v", res)
	return nil
}

func (db *appRepo) deleteApp(ctx context.Context, app *datastore.Application, update bson.D) error {
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
