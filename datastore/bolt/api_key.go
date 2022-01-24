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
	return a.createUpdateApplication(apiKey)
}

func (a *apiKeyRepo) createUpdateApplication(apiKey *datastore.APIKey) error {
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
	return a.createUpdateApplication(apiKey)
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
	var apiKeys []datastore.APIKey = make([]datastore.APIKey, 0, 1)
	var apiKey datastore.APIKey

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			err := json.Unmarshal(v, &apiKey)
			if err != nil {
				return err
			}

			if apiKey.MaskID == maskID {
				apiKeys = append(apiKeys, apiKey)
				break
			}
		}

		return nil
	})

	if err != nil && len(apiKeys) == 0 {
		return &apiKey, err
	}

	if err == nil && len(apiKeys) == 0 {
		return &apiKey, datastore.ErrAPIKeyNotFound
	}

	return &apiKeys[0], err
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
	var apiKeys []datastore.APIKey = make([]datastore.APIKey, 0, 1)
	var apiKey datastore.APIKey

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			err := json.Unmarshal(v, &apiKey)
			if err != nil {
				return err
			}

			if apiKey.Hash == hash {
				apiKeys = append(apiKeys, apiKey)
				break
			}
		}

		return nil
	})

	if err != nil && len(apiKeys) == 0 {
		return &apiKey, err
	}

	if err == nil && len(apiKeys) == 0 {
		return &apiKey, datastore.ErrAPIKeyNotFound
	}

	return &apiKeys[0], err
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

	if page < 1 {
		prevPage = 1
	} else {
		prevPage = page - 1
	}

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		c := b.Cursor()
		i := 1

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				continue
			}

			if i > perPage*prevPage && i <= perPage*page {
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

		total, err := a.countAPIKeys()
		if err != nil {
			return err
		}

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

func (a *apiKeyRepo) countAPIKeys() (int64, error) {
	i := int64(0)

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		c := b.Cursor()

		var apiKey datastore.APIKey
		for k, v := c.First(); k != nil; k, v = c.Next() {
			err := json.Unmarshal(v, &apiKey)
			if err != nil {
				return err
			}

			i++
		}

		return nil
	})

	return i, err
}
