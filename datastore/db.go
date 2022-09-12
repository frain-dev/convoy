package datastore

import (
	"context"
	"errors"
	"reflect"
	"time"

	cm "github.com/frain-dev/convoy/datastore/mongo"
	pager "github.com/gobeam/mongo-go-pagination"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrInvalidCollection = errors.New("Invalid collection type")

type CollectionKey string

const CollectionCtx CollectionKey = "collection"

type MongoStore struct {
	IsConnected bool
	Database    *mongo.Database
}

type Store interface {
	Save(ctx context.Context, payload interface{}, result interface{}) error
	SaveMany(ctx context.Context, payload []interface{}) error

	FindByID(ctx context.Context, id string, projection bson.M, result interface{}) error
	FindOne(ctx context.Context, filter, projection bson.M, result interface{}) error
	FindMany(ctx context.Context, filter, projection bson.M, sort interface{}, page, limit int64, results interface{}) (PaginationData, error)
	FindManyWithDeletedAt(ctx context.Context, filter, projection bson.M, sort interface{}, limit, skip int64, results interface{}) error
	FindAll(ctx context.Context, filter bson.M, sort interface{}, projection, results interface{}) error

	UpdateByID(ctx context.Context, id string, payload interface{}) error
	UpdateOne(ctx context.Context, filter bson.M, payload interface{}) error
	UpdateMany(ctx context.Context, filter, payload bson.M) error

	Inc(ctx context.Context, filter bson.M, payload interface{}) error

	DeleteByID(ctx context.Context, id string, hardDelete bool) error
	DeleteOne(ctx context.Context, filter bson.M, hardDelete bool) error
	DeleteMany(ctx context.Context, filter, payload bson.M, hardDelete bool) error

	Count(ctx context.Context, filter map[string]interface{}) (int64, error)

	Aggregate(ctx context.Context, pipeline mongo.Pipeline, result interface{}, allowDiskUse bool) error
}

// mongodb driver -> store (database) -> repo -> service -> handler

var _ Store = &MongoStore{}

/*
 * New
 * This initialises a new MongoDB repo for the collection
 */
func New(database *mongo.Database) Store {
	MongoStore := &MongoStore{
		IsConnected: true,
		Database:    database,
	}

	return MongoStore
}

var (
	ErrInvalidPtr = errors.New("out param is not a valid pointer")
)

func IsValidPointer(i interface{}) bool {
	v := reflect.ValueOf(i)
	return v.Type().Kind() == reflect.Ptr && !v.IsNil()
}

/**
 * Save
 * Save is used to save a record in the MongoStore
 */
func (d *MongoStore) Save(ctx context.Context, payload interface{}, out interface{}) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)
	result, err := collection.InsertOne(ctx, payload)

	if err != nil {
		return err
	}

	if out == nil {
		return nil
	}

	if !IsValidPointer(out) {
		return ErrInvalidPtr
	}

	return collection.FindOne(ctx, bson.M{"_id": result.InsertedID}).Decode(out)
}

/**
 * SaveMany
 * SaveMany is used to bulk insert into the MongoStore
 *
 * param: []interface{} payload
 * return: error
 */
func (d *MongoStore) SaveMany(ctx context.Context, payload []interface{}) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	_, err = collection.InsertMany(ctx, payload)
	return err
}

/**
 * FindByID
 * FindByID finds a single record by id in the MongoStore
 * returns nil if record is not found.
 *
 * param: interface{}            id
 * param: bson.M projection
 * return: bson.M
 */
func (d *MongoStore) FindByID(ctx context.Context, id string, projection bson.M, result interface{}) error {
	if !IsValidPointer(result) {
		return ErrInvalidPtr
	}
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	ops := options.FindOne()
	if projection != nil {
		ops.Projection = projection
	}

	return collection.FindOne(ctx, bson.M{"uid": id, "document_status": ActiveDocumentStatus}, ops).Decode(result)
}

/**
 * Find One by
 */
func (d *MongoStore) FindOne(ctx context.Context, filter, projection bson.M, result interface{}) error {
	if !IsValidPointer(result) {
		return ErrInvalidPtr
	}

	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	ops := options.FindOne()
	ops.Projection = projection

	filter["document_status"] = ActiveDocumentStatus

	return collection.FindOne(ctx, filter, ops).Decode(result)
}

func (d *MongoStore) FindMany(ctx context.Context, filter, projection bson.M, sort interface{}, page, limit, skip int64, results interface{}) (PaginationData, error) {
	if !IsValidPointer(results) {
		log.Errorf("Invalid Pointer Type")
		return PaginationData{}, ErrInvalidPtr
	}

	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return PaginationData{}, err
	}
	collection := d.Database.Collection(col)

	filter["document_status"] = ActiveDocumentStatus

	paginatedData, err := pager.
		New(collection).
		Context(ctx).
		Limit(limit).
		Page(page).
		Filter(filter).
		Sort("created_at", sort).
		Decode(&results).
		Find()

	if err != nil {
		return PaginationData{}, err
	}

	return PaginationData(paginatedData.Pagination), nil
}

func (d *MongoStore) FindManyWithDeletedAt(ctx context.Context, filter, projection bson.M, sort interface{}, limit, skip int64, results interface{}) error {
	if !IsValidPointer(results) {
		return ErrInvalidPtr
	}

	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	ops := options.Find()
	if limit > 0 {
		ops.Limit = &limit
	}
	if skip > 0 {
		ops.Skip = &skip
	}
	if projection != nil {
		ops.Projection = projection
	}
	if sort != nil {
		ops.Sort = sort
	}

	cursor, err := collection.Find(ctx, filter, ops)
	if err != nil {
		return err
	}

	return cursor.All(ctx, results)
}

func (d *MongoStore) FindAll(ctx context.Context, filter bson.M, sort interface{}, projection, results interface{}) error {
	if !IsValidPointer(results) {
		return ErrInvalidPtr
	}

	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	ops := options.Find()

	if projection != nil {
		ops.Projection = projection
	}

	if sort != nil {
		ops.Sort = sort
	}

	if filter == nil {
		filter = bson.M{}
	}

	filter["document_status"] = ActiveDocumentStatus

	cursor, err := collection.Find(ctx, filter, ops)
	if err != nil {
		return err
	}

	return cursor.All(ctx, results)
}

/**
 * UpdateByID
 * Updates a single record by id in the MongoStore
 *
 * param: interface{} id
 * param: interface{} payload
 * return: error
 */
func (d *MongoStore) UpdateByID(ctx context.Context, id string, payload interface{}) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	_, err = collection.UpdateOne(ctx, bson.M{"uid": id}, bson.M{"$set": payload}, nil)
	return err
}

func (d *MongoStore) UpdateOne(ctx context.Context, filter bson.M, payload interface{}) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	_, err = collection.UpdateOne(ctx, filter, bson.M{"$set": payload})
	return err
}

func (d *MongoStore) Inc(ctx context.Context, filter bson.M, payload interface{}) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	_, err = collection.UpdateOne(ctx, filter, bson.M{"$inc": payload})
	return err
}

/**
 * UpdateMany
 * Updates many items in the collection
 * `filter` this is the search criteria
 * `payload` this is the update payload.
 *
 * param: bson.M filter
 * param: interface{}            payload
 * return: error
 */
func (d *MongoStore) UpdateMany(ctx context.Context, filter, payload bson.M) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	_, err = collection.UpdateMany(ctx, filter, bson.M{"$set": payload})
	return err
}

/**
 * DeleteByID
 * Deletes a single record by id
 * where ID can be a string or whatever.
 *
 * param: interface{} id
 * param: bool hardDelete
 * return: error
 * If hard delete is false, a soft delete is executed where the document status is changed.
 * If hardDelete is true, the document is completely deleted.
 */
func (d *MongoStore) DeleteByID(ctx context.Context, id string, hardDelete bool) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	if hardDelete {
		_, err := collection.DeleteOne(ctx, bson.M{"uid": id}, nil)
		return err

	} else {
		payload := bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": DeletedDocumentStatus,
		}
		_, err := collection.UpdateOne(ctx, bson.M{"uid": id}, bson.M{"$set": payload}, nil)
		return err
	}
}

/**
 * DeleteOne
 * Deletes one item from the MongoStore using filter a hash map to properly filter what is to be deleted.
 *
 * param: bson.M filter
 * param: bool hardDelete
 * return: error
 * If hard delete is false, a soft delete is executed where the document status is changed.
 * If hardDelete is true, the document is completely deleted.
 */
func (d *MongoStore) DeleteOne(ctx context.Context, filter bson.M, hardDelete bool) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	if hardDelete {
		_, err := collection.DeleteOne(ctx, filter, nil)
		return err

	} else {
		payload := bson.M{
			"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
			"document_status": DeletedDocumentStatus,
		}
		_, err := collection.UpdateOne(ctx, filter, bson.M{"$set": payload})
		return err
	}
}

/**
 * DeleteMany
 * Hard Deletes many items in the collection
 * `filter` this is the search criteria
 *
 * param: bson.M filter
 * param: bool hardDelete
 * If hardDelete is false, a soft delete is executed where the document status is changed.
 * If hardDelete is true, the document is completely deleted.
 * return: error
 */
func (d *MongoStore) DeleteMany(ctx context.Context, filter, payload bson.M, hardDelete bool) error {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	if hardDelete {
		_, err := collection.DeleteMany(ctx, filter)
		return err
	} else {
		_, err := collection.UpdateMany(ctx, filter, bson.M{"$set": payload})
		return err
	}
}

func (d *MongoStore) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return 0, err
	}
	collection := d.Database.Collection(col)

	filter["document_status"] = ActiveDocumentStatus
	return collection.CountDocuments(ctx, filter)
}

func (d *MongoStore) Aggregate(ctx context.Context, pipeline mongo.Pipeline, output interface{}, allowDiskUse bool) error {
	if !IsValidPointer(output) {
		return ErrInvalidPtr
	}
	col, err := d.retrieveCollection(ctx)
	if err != nil {
		return err
	}
	collection := d.Database.Collection(col)

	opts := options.Aggregate()
	if allowDiskUse {
		opts.SetAllowDiskUse(true)
	}

	cur, err := collection.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return err
	}

	return cur.All(ctx, output)
}

func (d *MongoStore) retrieveCollection(ctx context.Context) (string, error) {
	switch ctx.Value(CollectionCtx) {
	case "configurations":
		return cm.ConfigCollection, nil
	case "groups":
		return cm.GroupCollection, nil
	case "organisations":
		return cm.OrganisationCollection, nil
	case "organisation_invites":
		return cm.OrganisationInvitesCollection, nil
	case "organisation_members":
		return cm.OrganisationMembersCollection, nil
	case "applications":
		return cm.AppCollection, nil
	case "events":
		return cm.EventCollection, nil
	case "sources":
		return cm.SourceCollection, nil
	case "subscriptions":
		return cm.SubscriptionCollection, nil
	case "eventdeliveries":
		return cm.EventDeliveryCollection, nil
	case "apiKeys":
		return cm.APIKeyCollection, nil
	default:
		return "", ErrInvalidCollection
	}
}
