package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/datastore"
	"go.mongodb.org/mongo-driver/mongo"
)

type deviceRepo struct {
	inner *mongo.Collection
	store datastore.Store
}

func NewDeviceRepository(db *mongo.Database, store datastore.Store) datastore.DeviceRepository {
	return &deviceRepo{
		inner: db.Collection(DeviceCollection),
		store: store,
	}
}

func (d *deviceRepo) CreateDevice(ctx context.Context, device *datastore.Device) error {
	device.ID = primitive.NewObjectID()
	return d.store.Save(ctx, device, nil)
}

func (d *deviceRepo) UpdateDevice(ctx context.Context, device *datastore.Device, appID, groupID string) error {
	filter := bson.M{
		"uid":             device.UID,
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	if !util.IsStringEmpty(appID) {
		filter["app_id"] = appID
	}

	device.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	update := bson.M{
		"updated_at":   device.UpdatedAt,
		"last_seen_at": device.LastSeenAt,
	}

	return d.store.UpdateOne(ctx, filter, update)
}

func (d *deviceRepo) UpdateDeviceLastSeen(ctx context.Context, device *datastore.Device, appID, groupID string) error {
	filter := bson.M{
		"uid":             device.UID,
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	if !util.IsStringEmpty(appID) {
		filter["app_id"] = appID
	}

	device.LastSeenAt = primitive.NewDateTimeFromTime(time.Now())
	device.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

	return d.store.UpdateOne(ctx, filter, device)
}

func (d *deviceRepo) DeleteDevice(ctx context.Context, uid string, appID, groupID string) error {
	filter := bson.M{
		"uid":             uid,
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	if !util.IsStringEmpty(appID) {
		filter["app_id"] = appID
	}

	return d.store.DeleteOne(ctx, filter)
}

func (d *deviceRepo) FetchDeviceByID(ctx context.Context, uid string, appID, groupID string) (*datastore.Device, error) {
	filter := bson.M{
		"uid":             uid,
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	if !util.IsStringEmpty(appID) {
		filter["app_id"] = appID
	}

	device := &datastore.Device{}
	err := d.store.FindOne(ctx, filter, nil, device)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, datastore.ErrDeviceNotFound
		}
		return nil, err
	}
	return device, nil
}
