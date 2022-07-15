package objectstore

import (
	"os"

	log "github.com/sirupsen/logrus"
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
