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

type endpointRepo struct {
	store datastore.Store
}

func NewEndpointRepo(store datastore.Store) datastore.EndpointRepository {
	return &endpointRepo{
		store: store,
	}
}

func (db *endpointRepo) CreateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, groupID string) error {
	ctx = db.setCollectionInContext(ctx)
	err := db.assertUniqueEndpointTitle(ctx, endpoint, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateEndpointName) {
			return err
		}

		return fmt.Errorf("failed to check if application name is unique: %v", err)
	}

	endpoint.ID = primitive.NewObjectID()
	if util.IsStringEmpty(endpoint.UID) {
		endpoint.UID = uuid.New().String()
	}

	return db.store.Save(ctx, endpoint, nil)
}

func (db *endpointRepo) FindEndpointByID(ctx context.Context, id string) (*datastore.Endpoint, error) {

	ctx = db.setCollectionInContext(ctx)
	endpoint := &datastore.Endpoint{}

	err := db.store.FindByID(ctx, id, nil, endpoint)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrEndpointNotFound
		return endpoint, err
	}

	if err != nil {
		return endpoint, err
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)

	filter := bson.M{"endpoint_id": endpoint.UID}
	count, err := db.store.Count(eventsCtx, filter)
	if err != nil {
		log.WithError(err).Errorf("failed to count events in %s", endpoint.UID)
		return endpoint, err
	}
	endpoint.Events = count

	return endpoint, err
}

func (db *endpointRepo) UpdateEndpoint(ctx context.Context, endpoint *datastore.Endpoint, groupID string) error {
	ctx = db.setCollectionInContext(ctx)

	err := db.assertUniqueEndpointTitle(ctx, endpoint, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateEndpointName) {
			return err
		}

		return fmt.Errorf("failed to check if endpoint name is unique: %v", err)
	}

	endpoint.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	update := bson.M{
		"$set": bson.M{
			"title":             endpoint.Title,
			"support_email":     endpoint.SupportEmail,
			"is_disabled":       endpoint.IsDisabled,
			"target_url":        endpoint.TargetURL,
			"secret":            endpoint.Secret,
			"description":       endpoint.Description,
			"slack_webhook_url": endpoint.SlackWebhookURL,
			"http_timeout":      endpoint.HttpTimeout,
			"rate_limit":        endpoint.RateLimit,
			"authentication":    endpoint.Authentication,
			"updated_at":        endpoint.UpdatedAt,
		},
	}

	return db.store.UpdateByID(ctx, endpoint.UID, update)
}

func (db *endpointRepo) DeleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint) error {
	ctx = db.setCollectionInContext(ctx)

	updateAsDeleted := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	err := db.store.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		err := db.deleteEndpointEvents(sessCtx, endpoint, updateAsDeleted)
		if err != nil {
			return err
		}

		err = db.deleteSubscription(sessCtx, endpoint, updateAsDeleted)
		if err != nil {
			return err
		}

		err = db.deleteEndpoint(sessCtx, endpoint, updateAsDeleted)
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

func (db *endpointRepo) CountGroupEndpoints(ctx context.Context, groupID string) (int64, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"group_id": groupID}

	count, err := db.store.Count(ctx, filter)
	if err != nil {
		log.WithError(err).Errorf("failed to count endpoints in group %s", groupID)
		return 0, err
	}
	return count, nil
}

func (db *endpointRepo) DeleteGroupEndpoints(ctx context.Context, groupID string) error {
	ctx = db.setCollectionInContext(ctx)

	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	return db.store.UpdateMany(ctx, bson.M{"group_id": groupID}, bson.M{"$set": update}, false)
}

func (db *endpointRepo) LoadEndpointsPaged(ctx context.Context, groupID, q string, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)
	filter := make(bson.M)

	if !util.IsStringEmpty(groupID) {
		filter["group_id"] = groupID
	}

	if !util.IsStringEmpty(q) {
		filter["title"] = bson.M{
			"$regex": primitive.Regex{Pattern: q, Options: "i"},
		}
	}

	var apps []datastore.Endpoint
	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &apps)

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	if apps == nil {
		apps = make([]datastore.Endpoint, 0)
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)
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

func (db *endpointRepo) LoadEndpointsPagedByGroupId(ctx context.Context, groupID string, pageable datastore.Pageable) ([]datastore.Endpoint, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"group_id": groupID}

	var endpoints []datastore.Endpoint
	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &endpoints)

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	if endpoints == nil {
		endpoints = make([]datastore.Endpoint, 0)
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)
	for i, app := range endpoints {
		filter = bson.M{"endpoint_id": app.UID}
		count, err := db.store.Count(eventsCtx, filter)
		if err != nil {
			log.Errorf("failed to count events in %s. Reason: %s", app.UID, err)
			return endpoints, datastore.PaginationData{}, err
		}
		endpoints[i].Events = count
	}

	return endpoints, pagination, nil
}

func (db *endpointRepo) SearchEndpointsByGroupId(ctx context.Context, groupId string, searchParams datastore.SearchParams) ([]datastore.Endpoint, error) {
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

	var endpoints []datastore.Endpoint

	_, err := db.store.FindMany(ctx, filter, nil, nil, 0, 0, &endpoints)
	if err != nil {
		return endpoints, err
	}

	eventsCtx := context.WithValue(context.Background(), datastore.CollectionCtx, datastore.EventCollection)
	for i, endpoint := range endpoints {
		filter = bson.M{"app_id": endpoint.UID}
		count, err := db.store.Count(eventsCtx, filter)
		if err != nil {
			log.Errorf("failed to count events in %s. Reason: %s", endpoint.UID, err)
			return endpoints, err
		}
		endpoints[i].Events = count
	}

	return endpoints, nil
}

func (db *endpointRepo) ExpireSecret(ctx context.Context, groupID, endpointID string, secrets []datastore.Secret) error {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"uid":             endpointID,
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	update := bson.M{
		"$set": bson.M{
			"secrets":    secrets,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	return db.store.UpdateOne(ctx, filter, update)
}

func (db *endpointRepo) assertUniqueEndpointTitle(ctx context.Context, endpoint *datastore.Endpoint, groupID string) error {
	ctx = db.setCollectionInContext(ctx)
	f := bson.M{
		"uid":      bson.M{"$ne": endpoint.UID},
		"title":    endpoint.Title,
		"group_id": groupID,
	}

	count, err := db.store.Count(ctx, f)

	if err != nil {
		return err
	}

	if count != 0 {
		return datastore.ErrDuplicateEndpointName
	}

	return nil
}

func (db *endpointRepo) deleteEndpointEvents(ctx context.Context, endpoint *datastore.Endpoint, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.EventCollection)

	filter := bson.M{"endpoint_id": endpoint.UID}
	err := db.store.UpdateMany(ctx, filter, update, true)

	return err
}

func (db *endpointRepo) deleteEndpoint(ctx context.Context, endpoint *datastore.Endpoint, update bson.M) error {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{"uid": endpoint.UID}
	err := db.store.UpdateMany(ctx, filter, update, true)
	if err != nil {
		return err
	}

	return nil
}

func (db *endpointRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.EndpointCollection)
}

func findEndpoint(endpoints *[]datastore.Endpoint, id string) (*datastore.Endpoint, error) {
	for _, endpoint := range *endpoints {
		if endpoint.UID == id && endpoint.DeletedAt == 0 {
			return &endpoint, nil
		}
	}
	return nil, datastore.ErrEndpointNotFound
}

func (db *endpointRepo) deleteSubscription(ctx context.Context, endpoint *datastore.Endpoint, update bson.M) error {
	ctx = context.WithValue(ctx, datastore.CollectionCtx, datastore.SubscriptionCollection)

	filter := bson.M{"app_id": endpoint.UID}
	err := db.store.UpdateMany(ctx, filter, update, true)

	return err
}
