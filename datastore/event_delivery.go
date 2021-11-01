package datastore

import (
	"context"

	"github.com/frain-dev/convoy"
	"go.mongodb.org/mongo-driver/mongo"
)

type eventDeliveryRepo struct {
	inner *mongo.Collection
}

const (
	EventDeliveryCollection = "eventdelivery"
)

func NewEventDeliveryRepository(db *mongo.Database) convoy.EventDeliveryRepository {
	return &eventDeliveryRepo{
		inner: db.Collection(EventDeliveryCollection),
	}
}

func (db *eventDeliveryRepo) CreateEventDelivery(ctx context.Context,
	eventDelivery *convoy.EventDelivery) error {
	return nil
}

func (db *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context,
	id string) (*convoy.EventDelivery, error) {
	return nil, nil
}

func (db *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context,
	e convoy.EventDelivery, status convoy.EventDeliveryStatus) error {
	return nil
}

func (db *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context,
	e convoy.EventDelivery, attempt convoy.EventAttempt) error {
	return nil
}
