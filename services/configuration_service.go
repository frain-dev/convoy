package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/pkg/log"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
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

func (c *ConfigService) CreateConfiguration(ctx context.Context, newConfig *models.Configuration) (*datastore.Configuration, error) {
	storagePolicy := newConfig.StoragePolicy.Transform()
	if storagePolicy == nil {
		storagePolicy = &datastore.DefaultStoragePolicy
	}

	config := &datastore.Configuration{
		UID:                ulid.Make().String(),
		StoragePolicy:      storagePolicy,
		IsAnalyticsEnabled: true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if newConfig.IsSignupEnabled != nil {
		config.IsSignupEnabled = *newConfig.IsSignupEnabled
	}

	err := c.configRepo.CreateConfiguration(ctx, config)
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return config, nil
}

func (c *ConfigService) UpdateConfiguration(ctx context.Context, config *models.Configuration) (*datastore.Configuration, error) {
	cfg, err := c.configRepo.LoadConfiguration(ctx)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load configuration")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	if config.IsAnalyticsEnabled != nil {
		cfg.IsAnalyticsEnabled = *config.IsAnalyticsEnabled
	}

	if config.IsSignupEnabled != nil {
		cfg.IsSignupEnabled = *config.IsSignupEnabled
	}

	if config.StoragePolicy != nil {
		cfg.StoragePolicy = config.StoragePolicy.Transform()
	}

	err = c.configRepo.UpdateConfiguration(ctx, cfg)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update configuration")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return cfg, nil
}
