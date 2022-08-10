package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
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
		"status":       device.Status,
		"host_name":    device.HostName,
		"updated_at":   device.UpdatedAt,
		"last_seen_at": device.LastSeenAt,
	}

	return d.store.UpdateOne(ctx, filter, update)
}

func (d *deviceRepo) UpdateDeviceLastSeen(ctx context.Context, device *datastore.Device, appID, groupID string, status datastore.DeviceStatus) error {
	filter := bson.M{
		"uid":             device.UID,
		"group_id":        groupID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	if !util.IsStringEmpty(appID) {
		filter["app_id"] = appID
	}

	device.Status = status
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

	return d.store.DeleteOne(ctx, filter, false)
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

func (d *deviceRepo) FetchDeviceByHostName(ctx context.Context, hostName string, appID, groupID string) (*datastore.Device, error) {
	filter := bson.M{
		"group_id":        groupID,
		"host_name":       hostName,
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

func (d *deviceRepo) LoadDevicesPaged(ctx context.Context, groupID string, f *datastore.ApiKeyFilter, pageable datastore.Pageable) ([]datastore.Device, datastore.PaginationData, error) {
	var devices []datastore.Device

	filter := bson.M{"document_status": datastore.ActiveDocumentStatus, "group_id": groupID}

	if !util.IsStringEmpty(f.AppID) {
		filter["app_id"] = f.AppID
	}

	paginatedData, err := pager.New(d.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&devices).Find()
	if err != nil {
		return devices, datastore.PaginationData{}, err
	}

	if devices == nil {
		devices = make([]datastore.Device, 0)
	}

	return devices, datastore.PaginationData(paginatedData.Pagination), nil
}
