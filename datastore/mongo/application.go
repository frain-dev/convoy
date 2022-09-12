package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type appRepo struct {
	store datastore.Store
}

func NewApplicationRepo(store datastore.Store) datastore.ApplicationRepository {
	return &appRepo{
		store: store,
	}
}

func (db *appRepo) CreateApplication(ctx context.Context, app *datastore.Application, groupID string) error {
	ctx = db.setCollectionInContext(ctx)
	err := db.assertUniqueAppTitle(ctx, app, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateAppName) {
			return err
		}

		return fmt.Errorf("failed to check if application name is unique: %v", err)
	}

	app.ID = primitive.NewObjectID()
	if util.IsStringEmpty(app.UID) {
		app.UID = uuid.New().String()
	}
	return db.store.Save(ctx, app, nil)
}

func (db *appRepo) LoadApplicationsPaged(ctx context.Context, groupID, q string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)
	var filter bson.M

	if !util.IsStringEmpty(groupID) {
		filter["group_id"] = groupID
	}

	if !util.IsStringEmpty(q) {
		filter["title"] = bson.M{
			"$regex": primitive.Regex{Pattern: q, Options: "i"},
		}
	}

	var apps []datastore.Application
	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &apps)

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	if apps == nil {
		apps = make([]datastore.Application, 0)
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, EventCollection)
	for i, app := range apps {
		filter = bson.M{"app_id": app.UID}
		count, err := db.store.Count(eventsCtx, filter)
		if err != nil {
			log.Errorf("failed to count events in %s. Reason: %s", app.UID, err)
			return apps, datastore.PaginationData{}, err
		}
		apps[i].Events = count
	}

	return apps, pagination, nil
}

func (db *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, groupID string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"group_id": groupID}

	var apps []datastore.Application
	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &apps)

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	if apps == nil {
		apps = make([]datastore.Application, 0)
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, EventCollection)
	for i, app := range apps {
		filter = bson.M{"app_id": app.UID}
		count, err := db.store.Count(eventsCtx, filter)
		if err != nil {
			log.Errorf("failed to count events in %s. Reason: %s", app.UID, err)
			return apps, datastore.PaginationData{}, err
		}
		apps[i].Events = count
	}

	return apps, pagination, nil
}

func (db *appRepo) CountGroupApplications(ctx context.Context, groupID string) (int64, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"group_id": groupID}

	count, err := db.store.Count(ctx, filter)
	if err != nil {
		log.WithError(err).Errorf("failed to count apps in group %s", groupID)
		return 0, err
	}
	return count, nil
}

func (db *appRepo) SearchApplicationsByGroupId(ctx context.Context, groupId string, searchParams datastore.SearchParams) ([]datastore.Application, error) {
	ctx = db.setCollectionInContext(ctx)

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = searchParams.CreatedAtStart
	}

	filter := bson.M{
		"group_id": groupId,
		"created_at": bson.M{
			"$gte": primitive.NewDateTimeFromTime(time.Unix(start, 0)),
			"$lte": primitive.NewDateTimeFromTime(time.Unix(end, 0)),
		},
	}

	var apps []datastore.Application

	_, err := db.store.FindMany(ctx, filter, nil, nil, 0, 0, &apps)
	if err != nil {
		return apps, err
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, EventCollection)
	for i, app := range apps {
		filter = bson.M{"app_id": app.UID}
		count, err := db.store.Count(eventsCtx, filter)
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

	ctx = db.setCollectionInContext(ctx)
	var app *datastore.Application

	err := db.store.FindByID(ctx, id, nil, app)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrApplicationNotFound
		return app, err
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, EventCollection)
	filter := bson.M{"app_id": app.UID}
	count, err := db.store.Count(eventsCtx, filter)
	if err != nil {
		log.WithError(err).Errorf("failed to count events in %s", app.UID)
		return app, err
	}
	app.Events = count

	return app, err
}

func (db *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*datastore.Endpoint, error) {
	ctx = db.setCollectionInContext(ctx)

	app, err := db.FindApplicationByID(ctx, appID)
	if err != nil {
		return nil, err
	}

	return findEndpoint(&app.Endpoints, endpointID)
}

func (db *appRepo) UpdateApplication(ctx context.Context, app *datastore.Application, groupID string) error {
	ctx = db.setCollectionInContext(ctx)

	err := db.assertUniqueAppTitle(ctx, app, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateAppName) {
			return err
		}

		return fmt.Errorf("failed to check if application name is unique: %v", err)
	}

	app.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	update := bson.M{
		"endpoints":     app.Endpoints,
		"updated_at":    app.UpdatedAt,
		"title":         app.Title,
		"support_email": app.SupportEmail,
		"is_disabled":   app.IsDisabled,
	}

	return db.store.UpdateByID(ctx, app.UID, update)
}

func (db *appRepo) CreateApplicationEndpoint(ctx context.Context, groupID string, appID string, endpoint *datastore.Endpoint) error {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"uid": appID, "document_status": datastore.ActiveDocumentStatus}
	update := bson.M{
		"$push": bson.M{
			"endpoints": endpoint,
		},
		"$set": bson.M{
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return db.store.UpdateOne(ctx, filter, update)
}

func (db *appRepo) DeleteGroupApps(ctx context.Context, groupID string) error {
	ctx = db.setCollectionInContext(ctx)

	update := bson.M{
		"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
		"document_status": datastore.DeletedDocumentStatus,
	}

	return db.store.UpdateMany(ctx, bson.M{"group_id": groupID}, update)
}

func (db *appRepo) DeleteApplication(ctx context.Context, app *datastore.Application) error {
	ctx = db.setCollectionInContext(ctx)

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

func (db *appRepo) assertUniqueAppTitle(ctx context.Context, app *datastore.Application, groupID string) error {
	ctx = db.setCollectionInContext(ctx)
	f := bson.M{
		"uid":      bson.M{"$ne": app.UID},
		"title":    app.Title,
		"group_id": groupID,
	}

	count, err := db.store.Count(ctx, f)
	if err != nil {
		return err
	}

	if count != 0 {
		return datastore.ErrDuplicateAppName
	}

	return nil
}

func (db *appRepo) updateMessagesInApp(ctx context.Context, app *datastore.Application, update bson.D) error {
	ctx = db.setCollectionInContext(ctx)

	var msgOperations []mongo.WriteModel

	updateMessagesOperation := mongo.NewUpdateManyModel()
	msgFilter := bson.M{"app_id": app.UID}
	updateMessagesOperation.SetFilter(msgFilter)
	updateMessagesOperation.SetUpdate(update)
	msgOperations = append(msgOperations, updateMessagesOperation)

	eventCollection := db.collection.Database().Collection(EventCollection)
	res, err := eventCollection.BulkWrite(ctx, msgOperations)
	if err != nil {
		log.Errorf("failed to delete messages in %s. Reason: %s", app.UID, err)
		return err
	}
	log.Infof("results of app messages op: %+v", res)
	return nil
}

func (db *appRepo) deleteApp(ctx context.Context, app *datastore.Application, update bson.D) error {
	ctx = db.setCollectionInContext(ctx)

	var appOperations []mongo.WriteModel
	updateAppOperation := mongo.NewUpdateOneModel()
	filter := bson.D{primitive.E{Key: "uid", Value: app.UID}}
	updateAppOperation.SetFilter(filter)
	updateAppOperation.SetUpdate(update)
	appOperations = append(appOperations, updateAppOperation)

	res, err := db.collection.BulkWrite(ctx, appOperations)
	if err != nil {
		log.Errorf("failed to delete app %s. Reason: %s", app.UID, err)
		return err
	}
	log.Infof("results of app op: %+v", res)
	return nil
}

func (db *appRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, AppCollection)
}

func findEndpoint(endpoints *[]datastore.Endpoint, id string) (*datastore.Endpoint, error) {
	for _, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			return &endpoint, nil
		}
	}
	return nil, datastore.ErrEndpointNotFound
}
