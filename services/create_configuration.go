package services

import (
	"context"
	"errors"
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
	if _, err := c.ConfigRepo.LoadConfiguration(ctx); err == nil {
		return nil, util.NewServiceError(http.StatusConflict, datastore.ErrConfigAlreadyExists)
	} else if !errors.Is(err, datastore.ErrConfigNotFound) {
		return nil, err
	}

	storagePolicy := c.NewConfig.StoragePolicy.Transform()
	if storagePolicy == nil {
		storagePolicy = &datastore.DefaultStoragePolicy
	}

	rc := c.NewConfig.RetentionPolicy.Transform()
	if rc == nil {
		rc = &datastore.DefaultRetentionPolicy
	}

	wa := c.NewConfig.WebhookArchiving.Transform()
	if wa == nil {
		wa = &datastore.DefaultWebhookArchiving
	}

	config := &datastore.Configuration{
		UID:                ulid.Make().String(),
		StoragePolicy:      storagePolicy,
		IsAnalyticsEnabled: true,
		RetentionPolicy:    rc,
		WebhookArchiving:   wa,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if c.NewConfig.IsSignupEnabled != nil {
		config.IsSignupEnabled = *c.NewConfig.IsSignupEnabled
	}

	err := c.ConfigRepo.CreateConfiguration(ctx, config)
	if err != nil {
		var se *util.ServiceError
		if errors.As(err, &se) && se.ErrCode() == http.StatusConflict {
			return nil, se
		}
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return config, nil
}
