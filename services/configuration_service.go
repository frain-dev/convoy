package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ConfigService struct {
	configRepo datastore.ConfigurationRepository
}

func NewConfigService(configRepo datastore.ConfigurationRepository) *ConfigService {
	return &ConfigService{configRepo: configRepo}
}

func (c *ConfigService) LoadConfiguration(ctx context.Context) ([]*datastore.Configuration, error) {
	configResponse := []*datastore.Configuration{}

	config, err := c.configRepo.LoadConfiguration(ctx)
	if err != nil {
		if errors.Is(err, datastore.ErrConfigNotFound) {
			return configResponse, nil
		}

		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	configResponse = append(configResponse, config)

	return configResponse, nil
}

func (c *ConfigService) CreateOrUpdateConfiguration(ctx context.Context, newConfig *models.Configuration) (*datastore.Configuration, error) {
	if err := util.Validate(newConfig); err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	config, err := c.configRepo.LoadConfiguration(ctx)
	if err != nil {
		if errors.Is(err, datastore.ErrConfigNotFound) {
			config := &datastore.Configuration{
				UID:            uuid.New().String(),
				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				DocumentStatus: datastore.ActiveDocumentStatus,
			}

			if newConfig.IsAnalyticsEnabled != nil {
				config.IsAnalyticsEnabled = *newConfig.IsAnalyticsEnabled
			}

			err := c.configRepo.CreateConfiguration(ctx, config)
			if err != nil {
				return nil, NewServiceError(http.StatusInternalServerError, err)
			}

			return config, nil
		}
		return nil, NewServiceError(http.StatusInternalServerError, err)
	}

	if newConfig.IsAnalyticsEnabled != nil {
		config.IsAnalyticsEnabled = *newConfig.IsAnalyticsEnabled
		err := c.configRepo.UpdateConfiguration(ctx, config)
		if err != nil {
			return nil, NewServiceError(http.StatusInternalServerError, err)
		}

		return config, nil
	}

	return config, nil
}
