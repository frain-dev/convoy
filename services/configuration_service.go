package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type ConfigService struct {
	configRepo datastore.ConfigurationRepository
}

func NewConfigService(configRepo datastore.ConfigurationRepository) *ConfigService {
	return &ConfigService{configRepo: configRepo}
}

func (c *ConfigService) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	config, err := c.configRepo.LoadConfiguration(ctx)
	if err != nil {
		if errors.Is(err, datastore.ErrConfigNotFound) {
			return config, nil
		}

		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return config, nil
}
