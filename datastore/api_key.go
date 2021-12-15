package datastore

import (
	"context"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type apiKeyRepo struct {
	client *mongo.Collection
}

const APIKeyCollection = "apiKeys"

func NewApiKeyRepo(db *mongo.Database) *apiKeyRepo {
	return &apiKeyRepo{client: db.Collection(APIKeyCollection)}
}

func (a *apiKeyRepo) CreateAPIKey(ctx context.Context, apiKey *convoy.APIKey) error {
	apiKey.ID = primitive.NewObjectID()

	if util.IsStringEmpty(apiKey.UID) {
		apiKey.UID = uuid.New().String()
	}

	_, err := a.client.InsertOne(ctx, apiKey)
	return err
}

func (a *apiKeyRepo) UpdateAPIKey(ctx context.Context, apiKey *convoy.APIKey) error {
	filter := bson.M{"uid": apiKey.UID}
	_, err := a.client.UpdateOne(ctx, filter, apiKey)
	return err
}

func (a *apiKeyRepo) FindAPIKeyByID(ctx context.Context, uid string) (*convoy.APIKey, error) {
	apiKey := &convoy.APIKey{}
	err := a.client.FindOne(ctx, bson.M{"uid": uid}).Decode(apiKey)
	return apiKey, err
}

func (a *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*convoy.APIKey, error) {
	apiKey := &convoy.APIKey{}
	err := a.client.FindOne(ctx, bson.M{"hash": hash}).Decode(apiKey)
	return apiKey, err
}

func (a *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, group string, pageable *models.Pageable) ([]convoy.APIKey, *pager.PaginationData, error) {
	var apiKeys []convoy.APIKey

	filter := bson.M{"group": group}

	paginatedData, err := pager.
		New(a.client).
		Context(ctx).
		Limit(int64(pageable.PerPage)).
		Page(int64(pageable.Page)).
		Sort("created_at", pageable.Sort).
		Filter(filter).
		Decode(&apiKeys).
		Find()

	if err != nil {
		return nil, nil, err
	}

	return apiKeys, &paginatedData.Pagination, nil
}

// TODO(daniel): i believe deleting it completely makes sense in this case
func (a *apiKeyRepo) DeleteAPIKey(ctx context.Context, uid string) error {
	_, err := a.client.DeleteOne(ctx, bson.M{"uid": uid})
	return err
}
