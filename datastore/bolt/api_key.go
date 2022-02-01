package bolt

import (
	"context"
	"errors"
	"math"

	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type apiKeyRepo struct {
	db *badgerhold.Store
}

func NewApiRoleRepo(db *badgerhold.Store) datastore.APIKeyRepository {
	return &apiKeyRepo{db: db}
}

func (a *apiKeyRepo) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	return a.db.Insert(apiKey.UID, apiKey)
}

func (a *apiKeyRepo) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	return a.db.Update(apiKey.UID, apiKey)
}

func (a *apiKeyRepo) FindAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	var apiKey datastore.APIKey

	err := a.db.Get(uid, &apiKey)

	if err != nil && errors.Is(err, badgerhold.ErrNotFound) {
		return &apiKey, datastore.ErrAPIKeyNotFound
	}

	return &apiKey, err
}

func (a *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	var apiKey datastore.APIKey

	err := a.db.FindOne(&apiKey, badgerhold.Where("MaskID").Eq(maskID))

	if err != nil && errors.Is(err, badgerhold.ErrNotFound) {
		return &apiKey, datastore.ErrAPIKeyNotFound
	}

	return &apiKey, err
}

func (a *apiKeyRepo) RevokeAPIKeys(ctx context.Context, uids []string) error {
	return a.db.DeleteMatching(&datastore.APIKey{}, badgerhold.Where("UID").In(badgerhold.Slice(uids)...))

}

func (a *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	var apiKey datastore.APIKey

	err := a.db.FindOne(&apiKey, badgerhold.Where("Hash").Eq(hash))

	if err != nil && errors.Is(err, badgerhold.ErrNotFound) {
		return &apiKey, datastore.ErrAPIKeyNotFound
	}

	return &apiKey, err
}

func (a *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	var apiKeys []datastore.APIKey = make([]datastore.APIKey, 0)

	page := pageable.Page
	perPage := pageable.PerPage
	data := datastore.PaginationData{}

	if pageable.Page < 1 {
		page = 1
	}

	if pageable.PerPage < 1 {
		perPage = 10
	}

	prevPage := page - 1
	lowerBound := perPage * prevPage

	q := &badgerhold.Query{}

	err := a.db.Find(&apiKeys, q.Skip(lowerBound).Limit(perPage))

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	total, err := a.db.Count(&datastore.APIKey{}, &badgerhold.Query{})

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	data.Total = int64(total)
	data.TotalPage = int64(math.Ceil(float64(total) / float64(perPage)))
	data.PerPage = int64(perPage)
	data.Next = int64(page + 1)
	data.Page = int64(page)
	data.Prev = int64(prevPage)

	return apiKeys, data, err
}
