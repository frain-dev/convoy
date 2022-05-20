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
}

func NewSubscriptionRepo(db *mongo.Database) datastore.SubscriptionRepository {
	return &subscriptionRepo{
		client: db.Collection(SubscriptionCollection),
	}
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.ID = primitive.NewObjectID()
	_, err := s.client.InsertOne(ctx, subscription)
	return err
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	if groupId != subscription.GroupID {
		return datastore.ErrNotAuthorisedToAccessDocument
	}

	subscription.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	filter := bson.D{primitive.E{Key: "uid", Value: subscription.UID}}

	update := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "name", Value: subscription.Name},
	}}}

	_, err := s.client.UpdateOne(ctx, filter, update)
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

	update := bson.M{
		"$set": bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": datastore.DeletedDocumentStatus,
		},
	}

	filter := bson.M{"uid": subscription.UID, "group_id": groupId}
	_, err := s.client.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (s *subscriptionRepo) FindSubscriptionByID(ctx context.Context, groupId string, uid string) (*datastore.Subscription, error) {
	var subscription *datastore.Subscription

	filter := bson.M{"uid": uid, "group_id": groupId}
	err := s.client.FindOne(ctx, filter).Decode(&subscription)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrSubscriptionNotFound
	}

	return subscription, err
}
