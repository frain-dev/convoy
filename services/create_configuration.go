package services

import (
	"context"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type CreateConfigService struct {
	ConfigRepo datastore.ConfigurationRepository
	NewConfig  *models.Configuration
}

func (c *CreateConfigService) Run(ctx context.Context) (*datastore.Configuration, error) {
	storagePolicy := c.NewConfig.StoragePolicy.Transform()
	if storagePolicy == nil {
		storagePolicy = &datastore.DefaultStoragePolicy
	}

	rc := c.NewConfig.RetentionPolicy.Transform()
	if rc == nil {
		rc = &datastore.DefaultRetentionPolicy
	}

	config := &datastore.Configuration{
		UID:                ulid.Make().String(),
		StoragePolicy:      storagePolicy,
		IsAnalyticsEnabled: true,
		RetentionPolicy:    rc,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if c.NewConfig.IsSignupEnabled != nil {
		config.IsSignupEnabled = *c.NewConfig.IsSignupEnabled
	}

	err := c.ConfigRepo.CreateConfiguration(ctx, config)
	if err != nil {
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return config, nil
}
