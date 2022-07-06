package objectstore

import (
	"errors"
	"os"
)

type OnPremClient struct {
	opts ObjectStoreOptions
}

func NewOnPremClient(opts ObjectStoreOptions) (ObjectStore, error) {
	if opts.OnPremStorageDir == "" {
		return nil, errors.New("please provide path to on-prem storage")
	}

	client := &OnPremClient{
		opts: opts,
	}

	return client, nil

}

func (o *OnPremClient) Save(filename string) error {
	if _, err := os.Stat(filename); err == nil {
		return err

	} else if errors.Is(err, os.ErrNotExist) {
		return os.ErrNotExist

	} else {
		return err
	}
}
