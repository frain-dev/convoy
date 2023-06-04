package services

import (
	"context"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type UpdateConfigService struct {
	ConfigRepo datastore.ConfigurationRepository
	Config     *models.Configuration
}

func (c *UpdateConfigService) Run(ctx context.Context) (*datastore.Configuration, error) {
	cfg, err := c.ConfigRepo.LoadConfiguration(ctx)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load configuration")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
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

	err = c.ConfigRepo.UpdateConfiguration(ctx, cfg)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update configuration")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return cfg, nil
}
