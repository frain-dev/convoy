package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type apiKeyRepo struct {
	store datastore.Store
}

func NewApiKeyRepo(store datastore.Store) datastore.APIKeyRepository {
	return &apiKeyRepo{
		store: store,
	}
}

func (db *apiKeyRepo) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	ctx = db.setCollectionInContext(ctx)

	if util.IsStringEmpty(apiKey.UID) {
		apiKey.UID = ulid.Make().String()
	}

	return db.store.Save(ctx, apiKey, nil)
}

func (db *apiKeyRepo) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	ctx = db.setCollectionInContext(ctx)

	update := bson.M{
		"$set": apiKey,
	}

	return db.store.UpdateByID(ctx, apiKey.UID, update)
}

func (db *apiKeyRepo) FindAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	ctx = db.setCollectionInContext(ctx)

	apiKey := &datastore.APIKey{}

	err := db.store.FindByID(ctx, uid, nil, apiKey)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, nil
}

func (db *apiKeyRepo) FindAPIKeyByProjectID(ctx context.Context, projectID string) (*datastore.APIKey, error) {
	ctx = db.setCollectionInContext(ctx)

	apiKey := &datastore.APIKey{}

	err := db.store.FindOne(ctx, bson.M{"role.project": projectID}, nil, apiKey)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, nil
}

func (db *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	ctx = db.setCollectionInContext(ctx)
	apiKey := new(datastore.APIKey)

	filter := bson.M{
		"mask_id": maskID,
	}

	err := db.store.FindOne(ctx, filter, nil, apiKey)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, err
}

func (db *apiKeyRepo) RevokeAPIKeys(ctx context.Context, uids []string) error {
	ctx = db.setCollectionInContext(ctx)

	filter := bson.M{
		"uid": bson.M{
			"$in": uids,
		},
	}

	updateAsDeleted := bson.M{"deleted_at": primitive.NewDateTimeFromTime(time.Now())}

	return db.store.UpdateMany(ctx, filter, bson.M{"$set": updateAsDeleted}, false)
}

func (db *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	ctx = db.setCollectionInContext(ctx)
	apiKey := &datastore.APIKey{}

	filter := bson.M{"hash": hash}

	err := db.store.FindOne(ctx, filter, nil, apiKey)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = datastore.ErrAPIKeyNotFound
		}
		return nil, err
	}

	return apiKey, err
}

func (db *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, f *datastore.ApiKeyFilter, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)

	var apiKeys []datastore.APIKey

	filter := bson.M{}

	if !util.IsStringEmpty(f.ProjectID) {
		filter["role.project"] = f.ProjectID // TODO(daniel): migrate apikey.group field
	}

	if !util.IsStringEmpty(f.EndpointID) {
		filter["role.endpoint"] = f.EndpointID
	}

	if !util.IsStringEmpty(string(f.KeyType)) {
		filter["key_type"] = f.KeyType
	}

	if !util.IsStringEmpty(f.UserID) {
		filter["user_id"] = f.UserID
	}

	if len(f.EndpointIDs) > 0 {
		filter["role.endpoint"] = bson.M{"$in": f.EndpointIDs}
	}

	pagination, err := db.store.FindMany(ctx, filter, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &apiKeys)
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	return apiKeys, pagination, nil
}

func (db *apiKeyRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.APIKeyCollection)
}
