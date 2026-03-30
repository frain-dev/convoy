package objectstore

import (
	"fmt"
	"os"

	log "github.com/frain-dev/convoy/pkg/logger"
)

type OnPremClient struct {
	opts   ObjectStoreOptions
	logger log.Logger
}

func NewOnPremClient(opts ObjectStoreOptions, logger log.Logger) (ObjectStore, error) {
	client := &OnPremClient{
		opts:   opts,
		logger: logger,
	}
	return client, nil
}

func (o *OnPremClient) Save(filename string) error {
	if _, err := os.Stat(filename); err != nil {
		return err
	}
	o.logger.Info(fmt.Sprintf("Successfully saved %q \n", filename))
	return nil
}
