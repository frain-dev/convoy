package objectstore

import (
	"os"

	"github.com/frain-dev/convoy/pkg/log"
)

type OnPremClient struct {
	opts ObjectStoreOptions
}

func NewOnPremClient(opts ObjectStoreOptions) (ObjectStore, error) {
	client := &OnPremClient{
		opts: opts,
	}
	return client, nil

}

func (o *OnPremClient) Save(filename string) error {
	if _, err := os.Stat(filename); err != nil {
		return err
	}
	log.Printf("Successfully saved %q \n", filename)
	return nil
}
