package services

import (
	"context"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type UpdateConfigService struct {
	ConfigRepo datastore.ConfigurationRepository
	Config     *models.Configuration
}

func (c *UpdateConfigService) Run(ctx context.Context) (*datastore.Configuration, error) {
	cfg, err := c.ConfigRepo.LoadConfiguration(ctx)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load configuration")
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if c.Config.IsAnalyticsEnabled != nil {
		cfg.IsAnalyticsEnabled = *c.Config.IsAnalyticsEnabled
	}

	if c.Config.IsSignupEnabled != nil {
		cfg.IsSignupEnabled = *c.Config.IsSignupEnabled
	}

	if c.Config.StoragePolicy != nil {
		cfg.StoragePolicy = c.Config.StoragePolicy.Transform()
	}

	if c.Config.RetentionPolicy != nil {
		cfg.RetentionPolicy = c.Config.RetentionPolicy.Transform()
	}

	err = c.ConfigRepo.UpdateConfiguration(ctx, cfg)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update configuration")
		return nil, &ServiceError{ErrMsg: "failed to update configuration"}
	}

	return cfg, nil
}
