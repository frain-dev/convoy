package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type eventDeliveryRepo struct {
	inner *mongo.Collection
}

const (
	EventDeliveryCollection = "eventdeliveries"
)

func NewEventDeliveryRepository(db *mongo.Database) convoy.EventDeliveryRepository {
	return &eventDeliveryRepo{
		inner: db.Collection(EventDeliveryCollection),
	}
}

func (db *eventDeliveryRepo) CreateEventDelivery(ctx context.Context,
	eventDelivery *convoy.EventDelivery) error {

	eventDelivery.ID = primitive.NewObjectID()
	if util.IsStringEmpty(eventDelivery.UID) {
		eventDelivery.UID = uuid.New().String()
	}

	_, err := db.inner.InsertOne(ctx, eventDelivery)
	return err
}

func (db *eventDeliveryRepo) FindEventDeliveryByID(ctx context.Context,
	id string) (*convoy.EventDelivery, error) {
	e := new(convoy.EventDelivery)

	filter := bson.M{"uid": id, "document_status": bson.M{"$ne": convoy.DeletedDocumentStatus}}

	err := db.inner.FindOne(ctx, filter).Decode(&e)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = convoy.ErrEventDeliveryNotFound
	}

	return e, err
}

func (db *eventDeliveryRepo) UpdateStatusOfEventDelivery(ctx context.Context,
	e convoy.EventDelivery, status convoy.EventDeliveryStatus) error {

	filter := bson.M{"uid": e.UID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": primitive.NewDateTimeFromTime(time.Now()),
		},
	}

	result := db.inner.FindOneAndUpdate(ctx, filter, update)
	err := result.Err()
	if err != nil {
		log.WithError(err).Error("Failed to update event delivery status")
		return err
	}

	return nil
}

func (db *eventDeliveryRepo) UpdateEventDeliveryWithAttempt(ctx context.Context,
	e convoy.EventDelivery, attempt convoy.DeliveryAttempt) error {

	filter := bson.M{"uid": e.UID}
	update := bson.M{
		"$set": bson.M{
			"status":      e.Status,
			"description": e.Description,
			"metadata":    e.Metadata,
			"updated_at":  primitive.NewDateTimeFromTime(time.Now()),
		},
		"$push": bson.M{
			"attempts": attempt,
		},
	}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	if err != nil {
		log.WithError(err).Error("error updating an event delivery %s -%s\n", e.UID, err)
		return err
	}

	return nil
}
