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
	_, err := db.client.UpdateOne(ctx, filter, apiKey)
	return err
}

func (db *apiKeyRepo) FindAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	apiKey := &datastore.APIKey{}
	err := db.client.FindOne(ctx, bson.M{"uid": uid}).Decode(apiKey)
	return apiKey, err
}

func (db *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	apiKey := new(datastore.APIKey)
	err := db.client.FindOne(ctx, bson.M{"mask_id": maskID}).Decode(apiKey)

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
	err := db.client.FindOne(ctx, bson.M{"hash": hash}).Decode(apiKey)
	return apiKey, err
}

func (db *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	var apiKeys []datastore.APIKey

	filter := bson.M{"$or": bson.A{
		bson.M{"document_status": bson.M{"$ne": datastore.DeletedDocumentStatus}},
	}}

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
