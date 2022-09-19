package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type apiKeyRepo struct {
	innerDB *mongo.Database
	client  *mongo.Collection
}

const APIKeyCollection = "apiKeys"

func NewApiKeyRepo(client *mongo.Database) datastore.APIKeyRepository {
	return &apiKeyRepo{
		innerDB: client,
		client:  client.Collection(APIKeyCollection, nil),
	}
}

func (db *apiKeyRepo) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	apiKey.ID = primitive.NewObjectID()

	if util.IsStringEmpty(apiKey.UID) {
		apiKey.UID = uuid.New().String()
	}

	_, err := db.client.InsertOne(ctx, apiKey)
	return err
}

func (db *apiKeyRepo) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	filter := bson.M{"uid": apiKey.UID}

	update := bson.M{
		"$set": apiKey,
	}

	_, err := db.client.UpdateOne(ctx, filter, update)
	return err
}

func (db *apiKeyRepo) FindAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}

	filter := bson.M{
		"uid":             uid,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := db.client.FindOne(ctx, filter).Decode(apiKey)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrAPIKeyNotFound
	}

	return apiKey, err
}

func (db *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	apiKey := new(datastore.APIKey)

	filter := bson.M{
		"mask_id":         maskID,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := db.client.FindOne(ctx, filter).Decode(apiKey)

	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrAPIKeyNotFound
	}

	return apiKey, err
}

func (db *apiKeyRepo) RevokeAPIKeys(ctx context.Context, uids []string) error {
	filter := bson.M{
		"uid": bson.M{
			"$in": uids,
		},
	}

	updateAsDeleted := bson.D{primitive.E{Key: "$set", Value: bson.D{
		primitive.E{Key: "deleted_at", Value: primitive.NewDateTimeFromTime(time.Now())},
		primitive.E{Key: "document_status", Value: datastore.DeletedDocumentStatus},
	}}}

	_, err := db.client.UpdateMany(ctx, filter, updateAsDeleted)
	return err
}

func (db *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}

	filter := bson.M{
		"hash":            hash,
		"document_status": datastore.ActiveDocumentStatus,
	}

	err := db.client.FindOne(ctx, filter).Decode(apiKey)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = datastore.ErrAPIKeyNotFound
	}

	return apiKey, err
}

func (db *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, f *datastore.ApiKeyFilter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	var apiKeys []datastore.APIKey

	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	if !util.IsStringEmpty(f.GroupID) {
		filter["role.group"] = f.GroupID
	}

	if !util.IsStringEmpty(f.AppID) {
		filter["role.app"] = f.AppID
	}

	if !util.IsStringEmpty(string(f.KeyType)) {
		filter["key_type"] = f.KeyType
	}

	if !util.IsStringEmpty(f.UserID) {
		filter["user_id"] = f.UserID
	}

	paginatedData, err := pager.
		New(db.client).
		Context(ctx).
		Limit(int64(pageable.PerPage)).
		Page(int64(pageable.Page)).
		Filter(filter).
		Sort("created_at", pageable.Sort).
		Decode(&apiKeys).
		Find()
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	return apiKeys, datastore.PaginationData(paginatedData.Pagination), nil
}
