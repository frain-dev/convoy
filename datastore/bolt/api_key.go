package bolt

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"go.etcd.io/bbolt"
)

type apiKeyRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewApiRoleRepo(db *bbolt.DB) datastore.APIKeyRepository {
	bucketName := "api_keys"
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	if err != nil {
		return nil
	}

	return &apiKeyRepo{db: db, bucketName: bucketName}
}

func (e *apiKeyRepo) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	return nil
}

func (e *apiKeyRepo) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	return nil
}

func (e *apiKeyRepo) FindAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	return nil, nil
}

func (e *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	return nil, nil
}

func (e *apiKeyRepo) RevokeAPIKeys(ctx context.Context, uids []string) error {
	return nil
}

func (e *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	return nil, nil
}

func (e *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	return []datastore.APIKey{}, datastore.PaginationData{}, nil
}
