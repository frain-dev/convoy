package bolt

import (
	"context"
	"encoding/json"
	"math"

	"github.com/frain-dev/convoy/datastore"
	"go.etcd.io/bbolt"
)

const apiKeyBucketName = "apiKeys"

type apiKeyRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewApiRoleRepo(db *bbolt.DB) datastore.APIKeyRepository {
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(apiKeyBucketName))
		return err
	})

	if err != nil {
		return nil
	}

	return &apiKeyRepo{db: db, bucketName: apiKeyBucketName}
}

func (a *apiKeyRepo) CreateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	return a.createUpdateAPIKey(apiKey)
}

func (a *apiKeyRepo) createUpdateAPIKey(apiKey *datastore.APIKey) error {
	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		aJson, err := json.Marshal(apiKey)
		if err != nil {
			return err
		}

		err = b.Put([]byte(apiKey.UID), aJson)
		if err != nil {
			return err
		}

		return nil
	})
}

func (a *apiKeyRepo) UpdateAPIKey(ctx context.Context, apiKey *datastore.APIKey) error {
	return a.createUpdateAPIKey(apiKey)
}

func (a *apiKeyRepo) FindAPIKeyByID(ctx context.Context, uid string) (*datastore.APIKey, error) {
	var apiKey datastore.APIKey

	err := a.db.View(func(tx *bbolt.Tx) error {
		buf := tx.Bucket([]byte(a.bucketName)).Get([]byte(uid))

		if buf == nil {
			return datastore.ErrAPIKeyNotFound
		}

		err := json.Unmarshal(buf, &apiKey)
		if err != nil {
			return err
		}

		return nil
	})

	return &apiKey, err
}

func (a *apiKeyRepo) FindAPIKeyByMaskID(ctx context.Context, maskID string) (*datastore.APIKey, error) {
	var apiKey *datastore.APIKey

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var temp *datastore.APIKey
			err := json.Unmarshal(v, &temp)
			if err != nil {
				return err
			}

			if temp.MaskID == maskID {
				apiKey = temp
				return nil
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	if apiKey == nil {
		return nil, datastore.ErrAPIKeyNotFound
	}

	return apiKey, err
}

func (a *apiKeyRepo) RevokeAPIKeys(ctx context.Context, uids []string) error {
	err := a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		for _, uid := range uids {
			err := b.Delete([]byte(uid))
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (a *apiKeyRepo) FindAPIKeyByHash(ctx context.Context, hash string) (*datastore.APIKey, error) {
	var apiKey *datastore.APIKey

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var temp *datastore.APIKey
			err := json.Unmarshal(v, &temp)
			if err != nil {
				return err
			}

			if temp.Hash == hash {
				apiKey = temp
				return nil
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	if apiKey == nil {
		return nil, datastore.ErrAPIKeyNotFound
	}

	return apiKey, err
}

func (a *apiKeyRepo) LoadAPIKeysPaged(ctx context.Context, pageable *datastore.Pageable) ([]datastore.APIKey, datastore.PaginationData, error) {
	var apiKeys []datastore.APIKey = make([]datastore.APIKey, 0)

	page := pageable.Page
	prevPage := pageable.Page
	perPage := pageable.PerPage
	data := datastore.PaginationData{}

	if pageable.Page < 1 {
		page = 1
	}

	if pageable.PerPage < 1 {
		perPage = 10
	}

	prevPage = page - 1
	lowerBound := perPage * prevPage
	upperBound := perPage * page

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		c := b.Cursor()
		i := 1

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				continue
			}

			if i > lowerBound && i <= upperBound {
				var apiKey datastore.APIKey
				err := json.Unmarshal(v, &apiKey)
				if err != nil {
					return err
				}

				apiKeys = append(apiKeys, apiKey)
			}
			i++

			if i == (perPage*page)+perPage {
				break
			}
		}

		total := int64(b.Stats().KeyN)

		data.Total = total
		data.TotalPage = int64(math.Ceil(float64(total) / float64(perPage)))
		data.PerPage = int64(perPage)
		data.Next = int64(page + 1)
		data.Page = int64(page)
		data.Prev = int64(prevPage)

		return nil
	})

	return apiKeys, data, err
}
