package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type subscriptionRepo struct {
	client *mongo.Collection
	store  datastore.Store
}

func NewSubscriptionRepo(db *mongo.Database, store datastore.Store) datastore.SubscriptionRepository {
	return &subscriptionRepo{
		client: db.Collection(SubscriptionCollection),
		store:  store,
	}
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.ID = primitive.NewObjectID()
	return s.store.Save(ctx, subscription, nil)
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.M{
		"uid":             subscription.UID,
		"group_id":        groupId,
		"document_status": datastore.ActiveDocumentStatus,
	}

	update := bson.M{
		"name":        subscription.Name,
		"source_id":   subscription.SourceID,
		"endpoint_id": subscription.EndpointID,

		"filter_config.event_types": subscription.FilterConfig.EventTypes,
		"alert_config.count":        subscription.AlertConfig.Count,
		"alert_config.threshold":    subscription.AlertConfig.Threshold,

		"retry_config.type":        string(subscription.RetryConfig.Type),
		"retry_config.duration":    subscription.RetryConfig.Duration,
		"retry_config.retry_count": subscription.RetryConfig.RetryCount,
	}

	err := s.store.UpdateOne(ctx, filter, update)
	return err
}

func (s *subscriptionRepo) LoadSubscriptionsPaged(ctx context.Context, groupId string, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	filter := bson.M{"group_id": groupId, "document_status": datastore.ActiveDocumentStatus}

	var subscriptions []datastore.Subscription
	paginatedData, err := pager.
		New(s.client).
		Context(ctx).
		Limit(int64(pageable.PerPage)).
		Page(int64(pageable.Page)).
		Sort("created_at", -1).
		Filter(filter).
		Decode(&subscriptions).
		Find()

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	return subscriptions, datastore.PaginationData(paginatedData.Pagination), nil
}

func (s *subscriptionRepo) DeleteSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	filter := bson.M{
		"uid":      subscription.UID,
		"group_id": groupId,
	}
	return s.store.DeleteOne(ctx, filter)
}

func (s *subscriptionRepo) FindSubscriptionByID(ctx context.Context, groupId string, uid string) (*datastore.Subscription, error) {
	subscription := &datastore.Subscription{}

	filter := bson.M{"uid": uid, "group_id": groupId, "document_status": datastore.ActiveDocumentStatus}
	err := s.store.FindOne(ctx, filter, nil, subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrSubscriptionNotFound
	}

	return subscription, err
}

func (s *subscriptionRepo) FindSubscriptionsByEventType(ctx context.Context, groupId string, appId string, eventType datastore.EventType) ([]datastore.Subscription, error) {
	filter := bson.M{"group_id": groupId, "app_id": appId, "filter_config.event_types": string(eventType), "document_status": datastore.ActiveDocumentStatus}

	subscriptions := make([]datastore.Subscription, 0)
	err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionsByAppID(ctx context.Context, groupId string, appID string) ([]datastore.Subscription, error) {
	filter := bson.M{
		"app_id":          appID,
		"group_id":        groupId,
		"document_status": datastore.ActiveDocumentStatus,
	}

	subscriptions := make([]datastore.Subscription, 0)
	err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrSubscriptionNotFound
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionsBySourceIDs(ctx context.Context, groupId string, sourceId string) ([]datastore.Subscription, error) {
	filter := bson.M{"group_id": groupId, "source_id": sourceId, "document_status": datastore.ActiveDocumentStatus}

	subscriptions := make([]datastore.Subscription, 0)
	err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) UpdateSubscriptionStatus(ctx context.Context, groupId string, subscriptionId string, status datastore.SubscriptionStatus) error {
	filter := bson.M{
		"uid":             subscriptionId,
		"group_id":        groupId,
		"document_status": datastore.ActiveDocumentStatus,
	}

	update := bson.M{
		"status":     status,
		"updated_at": primitive.NewDateTimeFromTime(time.Now()),
	}

	err := s.store.UpdateOne(ctx, filter, update)
	return err
}
