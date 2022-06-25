package badger

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type subscriptionRepo struct {
	client *badgerhold.Store
}

func (s *subscriptionRepo) FindSubscriptionsByAppID(ctx context.Context, groupId string, appID string) ([]datastore.Subscription, error) {
	return nil, nil
}

func (*subscriptionRepo) UpdateSubscriptionStatus(context.Context, string, string, datastore.SubscriptionStatus) error {
	return nil
}

func (*subscriptionRepo) FindSubscriptionBySourceIDs(context.Context, string, string) ([]datastore.Subscription, error) {
	return nil, nil
}

func (*subscriptionRepo) FindSubscriptionByEventType(context.Context, string, string, datastore.EventType) ([]datastore.Subscription, error) {
	return nil, nil
}

func NewSubscriptionRepo(db *badgerhold.Store) datastore.SubscriptionRepository {
	return &subscriptionRepo{
		client: db,
	}
}

func (s *subscriptionRepo) CreateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	return nil
}

func (s *subscriptionRepo) UpdateSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	return nil
}

func (s *subscriptionRepo) LoadSubscriptionsPaged(ctx context.Context, groupId string, pageable datastore.Pageable) ([]datastore.Subscription, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}

func (s *subscriptionRepo) DeleteSubscription(ctx context.Context, groupId string, subscription *datastore.Subscription) error {
	return nil
}

func (s *subscriptionRepo) FindSubscriptionByID(ctx context.Context, groupId string, uid string) (*datastore.Subscription, error) {
	return nil, nil
}
