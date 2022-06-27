package datastore

import (
	"context"
	"errors"
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoStore struct {
	IsConnected    bool
	CollectionName string
	Collection     *mongo.Collection
	Database       *mongo.Database
}

type Store interface {
	Save(ctx context.Context, payload interface{}, result interface{}) error
	SaveMany(ctx context.Context, payload []interface{}) error

	FindByID(ctx context.Context, id string, projection bson.M, result interface{}) error
	FindOne(ctx context.Context, filter, projection bson.M, result interface{}) error
	FindMany(ctx context.Context, filter, projection bson.M, sort interface{}, limit, skip int64, results interface{}) error
	FindManyWithDeletedAt(ctx context.Context, filter, projection bson.M, sort interface{}, limit, skip int64, results interface{}) error
	FindAll(ctx context.Context, filter bson.M, sort interface{}, projection, results interface{}) error

	UpdateByID(ctx context.Context, id string, payload interface{}) error
	UpdateOne(ctx context.Context, filter bson.M, payload interface{}) error
	UpdateMany(ctx context.Context, filter, payload bson.M) error

	Inc(ctx context.Context, filter bson.M, payload interface{}) error

	DeleteByID(ctx context.Context, id string) error
	DeleteOne(ctx context.Context, filter bson.M) error

	Count(ctx context.Context, filter map[string]interface{}) (int64, error)

	Aggregate(ctx context.Context, pipeline mongo.Pipeline, result interface{}, allowDiskUse bool) error
}

// mongodb driver -> store (database) -> repo -> service -> handler

var _ Store = &mongoStore{}

/*
 * New
 * This initialises a new MongoDB repo for the collection
 */
func New(database *mongo.Database, collection string) Store {
	mongoStore := &mongoStore{
		IsConnected:    true,
		CollectionName: collection,
		Collection:     database.Collection(collection),
		Database:       database,
	}

	return mongoStore
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
 * Save is used to save a record in the mongoStore
 */
func (d *mongoStore) Save(ctx context.Context, payload interface{}, out interface{}) error {
	result, err := d.Collection.InsertOne(ctx, payload)

	if err != nil {
		return err
	}

	if out == nil {
		return nil
	}

	if !IsValidPointer(out) {
		return ErrInvalidPtr
	}

	return d.Collection.FindOne(ctx, bson.M{"_id": result.InsertedID}).Decode(out)
}

/**
 * SaveMany
 * SaveMany is used to bulk insert into the mongoStore
 *
 * param: []interface{} payload
 * return: error
 */
func (d *mongoStore) SaveMany(ctx context.Context, payload []interface{}) error {
	_, err := d.Collection.InsertMany(ctx, payload)
	return err
}

/**
 * FindByID
 * FindByID finds a single record by id in the mongoStore
 * returns nil if record is not found.
 *
 * param: interface{}            id
 * param: bson.M projection
 * return: bson.M
 */
func (d *mongoStore) FindByID(ctx context.Context, id string, projection bson.M, result interface{}) error {
	if !IsValidPointer(result) {
		return ErrInvalidPtr
	}

	ops := options.FindOne()
	if projection != nil {
		ops.Projection = projection
	}

	return d.Collection.FindOne(ctx, bson.M{"uid": id, "document_status": ActiveDocumentStatus}, ops).Decode(result)
}

/**
 * Find One by
 */
func (d *mongoStore) FindOne(ctx context.Context, filter, projection bson.M, result interface{}) error {
	if !IsValidPointer(result) {
		return ErrInvalidPtr
	}

	ops := options.FindOne()
	ops.Projection = projection

	filter["document_status"] = ActiveDocumentStatus

	return d.Collection.FindOne(ctx, filter, ops).Decode(result)
}

func (d *mongoStore) FindMany(ctx context.Context, filter, projection bson.M, sort interface{}, limit, skip int64, results interface{}) error {
	if !IsValidPointer(results) {
		return ErrInvalidPtr
	}

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

	filter["document_status"] = ActiveDocumentStatus

	cursor, err := d.Collection.Find(ctx, filter, ops)
	if err != nil {
		return err
	}

	return cursor.All(ctx, results)
}

func (d *mongoStore) FindManyWithDeletedAt(ctx context.Context, filter, projection bson.M, sort interface{}, limit, skip int64, results interface{}) error {
	if !IsValidPointer(results) {
		return ErrInvalidPtr
	}

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

	cursor, err := d.Collection.Find(ctx, filter, ops)
	if err != nil {
		return err
	}

	return cursor.All(ctx, results)
}

func (d *mongoStore) FindAll(ctx context.Context, filter bson.M, sort interface{}, projection, results interface{}) error {
	if !IsValidPointer(results) {
		return ErrInvalidPtr
	}

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

	cursor, err := d.Collection.Find(ctx, filter, ops)
	if err != nil {
		return err
	}

	return cursor.All(ctx, results)
}

/**
 * UpdateByID
 * Updates a single record by id in the mongoStore
 *
 * param: interface{} id
 * param: interface{} payload
 * return: error
 */
func (d *mongoStore) UpdateByID(ctx context.Context, id string, payload interface{}) error {
	_, err := d.Collection.UpdateOne(ctx, bson.M{"uid": id}, bson.M{"$set": payload}, nil)
	return err
}

func (d *mongoStore) UpdateOne(ctx context.Context, filter bson.M, payload interface{}) error {
	_, err := d.Collection.UpdateOne(ctx, filter, bson.M{"$set": payload})
	return err
}

func (d *mongoStore) Inc(ctx context.Context, filter bson.M, payload interface{}) error {
	_, err := d.Collection.UpdateOne(ctx, filter, bson.M{"$inc": payload})
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
func (d *mongoStore) UpdateMany(ctx context.Context, filter, payload bson.M) error {
	_, err := d.Collection.UpdateMany(ctx, filter, bson.M{"$set": payload})
	return err
}

/**
 * DeleteByID
 * Deletes a single record by id
 * where ID can be a string or whatever.
 * param: interface{} id
 * return: error
 * The record is not completed deleted, only the status is changed.
 */
func (d *mongoStore) DeleteByID(ctx context.Context, id string) error {
	payload := bson.M{
		"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
		"document_status": DeletedDocumentStatus,
	}

	_, err := d.Collection.UpdateOne(ctx, bson.M{"uid": id}, bson.M{"$set": payload}, nil)
	return err
}

/**
 * DeleteOne
 * Deletes one item from the mongoStore using filter a hash map to properly filter what is to be deleted.
 *
 * param: bson.M filter
 * return: error
 * The record is not completed deleted, only the status is changed.
 */
func (d *mongoStore) DeleteOne(ctx context.Context, filter bson.M) error {
	payload := bson.M{
		"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
		"document_status": DeletedDocumentStatus,
	}

	_, err := d.Collection.UpdateOne(ctx, filter, bson.M{"$set": payload})
	return err
}

func (d *mongoStore) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	filter["document_status"] = ActiveDocumentStatus
	return d.Collection.CountDocuments(ctx, filter)
}

func (d *mongoStore) Aggregate(ctx context.Context, pipeline mongo.Pipeline, output interface{}, allowDiskUse bool) error {
	if !IsValidPointer(output) {
		return ErrInvalidPtr
	}

	opts := options.Aggregate()
	if allowDiskUse {
		opts.SetAllowDiskUse(true)
	}

	cur, err := d.Collection.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return err
	}

	return cur.All(ctx, output)
}
