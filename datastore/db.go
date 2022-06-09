package datastore

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type mongoStore struct {
	IsConnected    bool
	CollectionName string
	Collection     *mongo.Collection
	Database       *mongo.Database
}

type Database interface {
	Save(ctx context.Context, payload interface{}, out interface{}) error
	SaveMany(ctx context.Context, payload []interface{}) error

	FindByID(ctx context.Context, id string, projection map[string]interface{}, result interface{}) error
	FindOne(ctx context.Context, filter, projection map[string]interface{}, result interface{}) error
	FindMany(ctx context.Context, filter, projection map[string]interface{}, sort interface{}, limit, skip int64, results interface{}) error
	FindManyWithDeletedAt(ctx context.Context, filter, projection map[string]interface{}, sort interface{}, limit, skip int64, results interface{}) error
	FindAll(ctx context.Context, filter map[string]interface{}, projection, results interface{}) error
	FindAllAdminRecords(ctx context.Context, results interface{}) error

	UpdateByID(ctx context.Context, id string, payload interface{}) error
	UpdateOne(ctx context.Context, filter map[string]interface{}, payload interface{}) error
	UpsertOne(ctx context.Context, filter map[string]interface{}, payload interface{}) error
	UpdateMany(ctx context.Context, filter, payload map[string]interface{}) error

	Inc(ctx context.Context, filter map[string]interface{}, payload interface{}) error

	DeleteByID(ctx context.Context, id string) error
	DeleteOne(ctx context.Context, filter map[string]interface{}) error
	DeleteMany(ctx context.Context, filter map[string]interface{}) error

	DestroyById(ctx context.Context, id interface{}) error
	DestroyOne(ctx context.Context, filter map[string]interface{}) error

	Aggregate(ctx context.Context, pipelines interface{}, result interface{}, allowDiskUse bool) error

	Count(ctx context.Context, filter map[string]interface{}) (int64, error)
	CountWithDeletedAt(ctx context.Context, filter map[string]interface{}) (int64, error)
}

var _ Database = &mongoStore{}

/*
 * New
 * This initialises a new MongoDB repo for the collection
 */
func New(database *mongo.Database, collection string) Database {
	mongoStore := mongoStore{
		IsConnected:    true,
		CollectionName: collection,
		Collection:     database.Collection(collection),
		Database:       database,
	}

	return &mongoStore
}

func IsValidPointer(i interface{}) bool {
	return reflect.ValueOf(i).Type().Kind() == reflect.Struct
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
		return errors.New("out param is not a valid pointer")
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

	if err != nil {
		return err
	}

	return nil
}

/**
 * FindByID
 * FindByID finds a single record by id in the mongoStore
 * returns nil if record is not found.
 *
 * param: interface{}            id
 * param: map[string]interface{} projection
 * return: map[string]interface{}
 */
func (d *mongoStore) FindByID(ctx context.Context, id string, projection map[string]interface{}, result interface{}) error {
	if result == nil {
		if !IsValidPointer(result) {
			return errors.New("result param is not a valid pointer")
		}
		return errors.New("result param should not be a nil pointer")
	}

	ops := options.FindOne()
	if projection != nil {
		ops.Projection = projection
	}

	if err := d.Collection.FindOne(ctx, bson.M{"_id": id, "document_status": ActiveDocumentStatus}, ops).Decode(result); err != nil {
		return err
	}
	return nil
}

/**
 * Find One by
 */
func (d *mongoStore) FindOne(ctx context.Context, filter, projection map[string]interface{}, result interface{}) error {
	if result == nil {
		return errors.New("result param should not be a nil pointer")
	}

	if !IsValidPointer(result) {
		return errors.New("result param is not a valid pointer")
	}
	ops := options.FindOne()
	ops.Projection = projection

	filter["document_status"] = ActiveDocumentStatus

	return d.Collection.FindOne(ctx, filter, ops).Decode(result)
}

func (d *mongoStore) FindMany(ctx context.Context, filter, projection map[string]interface{}, sort interface{}, limit, skip int64, results interface{}) error {
	if results == nil {
		return errors.New("results param should not be a nil pointer")
	}

	if !IsValidPointer(results) {
		return errors.New("results param is not a valid pointer")
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

	var output []map[string]interface{}
	for cursor.Next(ctx) { // there was nil here before
		var item map[string]interface{}
		_ = cursor.Decode(&item)
		output = append(output, item)
	}

	if b, e := json.Marshal(output); e == nil {
		_ = json.Unmarshal(b, &results)
	} else {
		return e
	}

	return nil
}

func (d *mongoStore) FindManyWithDeletedAt(ctx context.Context, filter, projection map[string]interface{}, sort interface{}, limit, skip int64, results interface{}) error {
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

	var output []map[string]interface{}
	for cursor.Next(ctx) {
		var item map[string]interface{}
		_ = cursor.Decode(&item)
		output = append(output, item)
	}

	if b, e := json.Marshal(output); e == nil {
		_ = json.Unmarshal(b, &results)
	} else {
		return e
	}

	return nil
}

func (d *mongoStore) FindAll(ctx context.Context, filter map[string]interface{}, projection, results interface{}) error {
	if results == nil {
		return errors.New("results param should not be a nil pointer")
	}

	ops := options.Find().SetSort(bson.M{"_id": -1})

	if projection != nil {
		ops.Projection = projection
	}

	filter["document_status"] = ActiveDocumentStatus

	cursor, err := d.Collection.Find(ctx, filter, ops)
	if err != nil {
		return err
	}

	var output []map[string]interface{}
	for cursor.Next(ctx) {
		var item map[string]interface{}
		if err := cursor.Decode(&item); err != nil {
			return err
		}

		output = append(output, item)
	}

	b, err := json.Marshal(output)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(b, &results); err != nil {
		return err
	}

	return nil
}

// FindAllAdminRecords retrieves all records including soft deleted data
// for admin
// Note: This is for admin use.
func (d *mongoStore) FindAllAdminRecords(ctx context.Context, results interface{}) error {

	ops := options.Find()

	cursor, err := d.Collection.Find(ctx, bson.D{}, ops)
	if err != nil {
		return err
	}

	var output []map[string]interface{}
	for cursor.Next(ctx) {
		var item map[string]interface{}
		_ = cursor.Decode(&item)
		output = append(output, item)
	}

	if b, e := json.Marshal(output); e == nil {
		_ = json.Unmarshal(b, &results)
	} else {
		return e
	}
	return nil
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
	result := d.Collection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{"$set": payload}, options.FindOneAndUpdate().SetUpsert(true))

	err := result.Err()
	if err != nil {
		return err
	}

	return nil
}

/**
 * UpsertOne
 *
 * Updates one item in the mongoStore using filter as the criteria.
 * If document does not exist in mongo, a new documents is automatically generated.
 *
 * param: map[string]interface{} filter
 * param: interface{}            payload
 * return: error
 */
func (d *mongoStore) UpsertOne(ctx context.Context, filter map[string]interface{}, payload interface{}) error {
	result := d.Collection.FindOneAndUpdate(ctx, filter, bson.M{
		"$set": payload,
	}, options.FindOneAndUpdate().SetUpsert(true))

	// If result gives a document does not exist error, record is inserted.
	if err := result.Err(); err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	return nil
}

//
func (d *mongoStore) UpdateOne(ctx context.Context, filter map[string]interface{}, payload interface{}) error {
	return d.Collection.FindOneAndUpdate(ctx, filter, bson.M{"$set": payload}).Err()
}

func (d *mongoStore) Inc(ctx context.Context, filter map[string]interface{}, payload interface{}) error {
	result := d.Collection.FindOneAndUpdate(ctx, filter, bson.M{"$inc": payload})
	return result.Err()
}

/**
 * UpdateMany
 * Updates many items in the collection
 * `filter` this is the search criteria
 * `payload` this is the update payload.
 *
 * param: map[string]interface{} filter
 * param: interface{}            payload
 * return: error
 */
func (d *mongoStore) UpdateMany(ctx context.Context, filter, payload map[string]interface{}) error {
	if _, err := d.Collection.UpdateMany(ctx, filter, bson.M{
		"$set": payload,
	}); err != nil {
		return err
	}
	return nil
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

	var u map[string]interface{}
	opts := options.FindOneAndUpdate()
	up := true
	opts.Upsert = &up

	payload := map[string]interface{}{
		"document_status": ActiveDocumentStatus,
	}

	if err := d.Collection.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{
		"$set": payload,
	}).Decode(&u); err != nil {
		return err
	}
	return nil
}

// DestroyById removes the record permanently from the db, should be used by admin only
func (d *mongoStore) DestroyById(ctx context.Context, id interface{}) error {
	var u map[string]interface{}
	if e := d.Collection.FindOneAndDelete(ctx, bson.M{
		"_id": id,
	}).Decode(&u); e != nil {
		return e
	}

	return nil
}

/**
 * DeleteOne
 * Deletes one item from the mongoStore using filter a hash map to properly filter what is to be deleted.
 *
 * param: map[string]interface{} filter
 * return: error
 * The record is not completed deleted, only the status is changed.
 */
func (d *mongoStore) DeleteOne(ctx context.Context, filter map[string]interface{}) error {
	var u map[string]interface{}
	opts := options.FindOneAndUpdate()
	up := true
	opts.Upsert = &up

	payload := map[string]interface{}{
		"document_status": ActiveDocumentStatus,
	}

	if err := d.Collection.FindOneAndUpdate(ctx, filter, bson.M{
		"$set": payload,
	}).Decode(&u); err != nil {
		return err
	}

	return nil
}

func (d *mongoStore) DestroyOne(ctx context.Context, filter map[string]interface{}) error {
	_, err := d.Collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Delete Many items from the mongoStore
 *
 * param: map[string]interface{} filter
 * return: error
 * The record is deleted completed here
 */
func (d *mongoStore) DeleteMany(ctx context.Context, filter map[string]interface{}) error {
	_, err := d.Collection.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

func (d *mongoStore) Count(ctx context.Context, filter map[string]interface{}) (int64, error) {
	filter["document_status"] = ActiveDocumentStatus
	return d.Collection.CountDocuments(ctx, filter)
}

func (d *mongoStore) CountWithDeletedAt(ctx context.Context, filter map[string]interface{}) (int64, error) {
	return d.Collection.CountDocuments(ctx, filter)
}

func (d *mongoStore) Aggregate(ctx context.Context, pipeline interface{}, output interface{}, allowDiskUse bool) error {
	opts := options.Aggregate()
	if allowDiskUse {
		opts.SetAllowDiskUse(true)
	}
	C, err := d.Collection.Aggregate(ctx, pipeline, opts)
	if err != nil {
		return err
	}
	if err := C.All(ctx, output); err != nil {
		return err
	}

	return nil
}
