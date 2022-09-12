package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
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

	apiKey.ID = primitive.NewObjectID()
	if util.IsStringEmpty(apiKey.UID) {
		apiKey.UID = uuid.New().String()
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

func (db *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	ctx = db.setCollectionInContext(ctx)
	apiKey := new(datastore.APIKey)

	filter := bson.M{
		"mask_id":         maskID,
		"document_status": datastore.ActiveDocumentStatus,
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

	updateAsDeleted := bson.M{
		"deleted_at":      primitive.NewDateTimeFromTime(time.Now()),
		"document_status": datastore.DeletedDocumentStatus,
	}

	return db.store.UpdateMany(ctx, filter, updateAsDeleted, false)
}

func (db *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	ctx = db.setCollectionInContext(ctx)
	apiKey := &datastore.APIKey{}

	filter := bson.M{
		"hash":            hash,
		"document_status": datastore.ActiveDocumentStatus,
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

func (db *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	ctx = db.setCollectionInContext(ctx)
	var apiKeys []datastore.APIKey

	pagination, err := db.store.FindMany(ctx, nil, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &apiKeys)

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	return apiKeys, pagination, nil
}

func (db *apiKeyRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.APIKeyCollection)
}
