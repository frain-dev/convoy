package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type subscriptionRepo struct {
	store datastore.Store
}

func NewSubscriptionRepo(store datastore.Store) datastore.SubscriptionRepository {
	return &subscriptionRepo{
		store: store,
	}
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.ID = primitive.NewObjectID()
	return s.store.Save(ctx, subscription, nil)
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
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

func (s *subscriptionRepo) LoadSubscriptionsPaged(ctx context.Context, groupId string, f *datastore.FilterBy, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{"group_id": groupId, "document_status": datastore.ActiveDocumentStatus}

	if !util.IsStringEmpty(f.AppID) {
		filter["app_id"] = f.AppID
	}

	var subscriptions []datastore.Subscription
	pagination, err := s.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &subscriptions)

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	return subscriptions, pagination, nil
}

func (s *subscriptionRepo) DeleteSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	ctx = s.setCollectionInContext(ctx)
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	filter := bson.M{
		"uid":      subscription.UID,
		"group_id": groupId,
	}
	return s.store.DeleteOne(ctx, filter, false)
}

func (s *subscriptionRepo) FindSubscriptionByID(ctx context.Context, groupId string, uid string) (*datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	subscription := &datastore.Subscription{}

	filter := bson.M{"uid": uid, "group_id": groupId, "document_status": datastore.ActiveDocumentStatus}
	err := s.store.FindOne(ctx, filter, nil, subscription)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrSubscriptionNotFound
	}

	return subscription, err
}

func (s *subscriptionRepo) FindSubscriptionsByEventType(ctx context.Context, groupId string, appId string, eventType datastore.EventType) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{"group_id": groupId, "app_id": appId, "filter_config.event_types": string(eventType), "document_status": datastore.ActiveDocumentStatus}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionsByAppID(ctx context.Context, groupId string, appID string) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{
		"app_id":          appID,
		"group_id":        groupId,
		"document_status": datastore.ActiveDocumentStatus,
	}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, datastore.ErrSubscriptionNotFound
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) FindSubscriptionsBySourceIDs(ctx context.Context, groupId string, sourceId string) ([]datastore.Subscription, error) {
	ctx = s.setCollectionInContext(ctx)
	filter := bson.M{"group_id": groupId, "source_id": sourceId, "document_status": datastore.ActiveDocumentStatus}

	subscriptions := make([]datastore.Subscription, 0)
	_, err := s.store.FindMany(ctx, filter, nil, nil, 0, 0, &subscriptions)
	if err != nil {
		return nil, err
	}

	return subscriptions, nil
}

func (s *subscriptionRepo) UpdateSubscriptionStatus(ctx context.Context, groupId string, subscriptionId string, status datastore.SubscriptionStatus) error {
	ctx = s.setCollectionInContext(ctx)
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
func (s *subscriptionRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.SubscriptionCollection)
}
