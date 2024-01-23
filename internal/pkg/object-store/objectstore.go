package objectstore

import (
	"errors"

	"github.com/frain-dev/convoy/datastore"
)

type ObjectStore interface {
	Save(string) error
}

type ObjectStoreOptions struct {
	Prefix           string
	Bucket           string
	AccessKey        string
	SecretKey        string
	Region           string
	Endpoint         string
	SessionToken     string
	OnPremStorageDir string
}

func NewObjectStoreClient(storage *datastore.StoragePolicyConfiguration) (ObjectStore, error) {
	switch storage.Type {
	case datastore.S3:
		objectStoreOpts := ObjectStoreOptions{
			Prefix:       storage.S3.Prefix.ValueOrZero(),
			Bucket:       storage.S3.Bucket.ValueOrZero(),
			Endpoint:     storage.S3.Endpoint.ValueOrZero(),
			AccessKey:    storage.S3.AccessKey.ValueOrZero(),
			SecretKey:    storage.S3.SecretKey.ValueOrZero(),
			SessionToken: storage.S3.SessionToken.ValueOrZero(),
			Region:       storage.S3.Region.ValueOrZero(),
		}

		objectStoreClient, err := NewS3Client(objectStoreOpts)
		if err != nil {
			return nil, err
		}
		return objectStoreClient, nil

	case datastore.OnPrem:
		exportDir := storage.OnPrem.Path
		objectStoreOpts := ObjectStoreOptions{
			OnPremStorageDir: exportDir.String,
		}
		objectStoreClient, err := NewOnPremClient(objectStoreOpts)
		if err != nil {
			return nil, err
		}
		return objectStoreClient, nil
	default:
		return nil, errors.New("invalid storage policy")
	}
}
