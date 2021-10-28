package datastore

import "go.mongodb.org/mongo-driver/mongo"

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
