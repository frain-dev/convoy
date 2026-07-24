package services

import (
	"context"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type UpdateConfigService struct {
	ConfigRepo datastore.ConfigurationRepository
	Config     *models.Configuration
	Logger     log.Logger
}

func (c *UpdateConfigService) Run(ctx context.Context) (*datastore.Configuration, error) {
	cfg, err := c.ConfigRepo.LoadConfiguration(ctx)
	if err != nil {
		c.Logger.ErrorContext(ctx, "failed to load configuration", "error", err)
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if c.Config.IsAnalyticsEnabled != nil {
		cfg.IsAnalyticsEnabled = *c.Config.IsAnalyticsEnabled
	}

	if c.Config.IsSignupEnabled != nil {
		cfg.IsSignupEnabled = *c.Config.IsSignupEnabled
	}

	if c.Config.StoragePolicy != nil {
		prevStorage := cfg.StoragePolicy
		cfg.StoragePolicy = c.Config.StoragePolicy.Transform()
		preserveStoragePolicySecrets(cfg.StoragePolicy, prevStorage)
	}

	if c.Config.RetentionPolicy != nil {
		cfg.RetentionPolicy = c.Config.RetentionPolicy.Transform()
	}

	err = c.ConfigRepo.UpdateConfiguration(ctx, cfg)
	if err != nil {
		c.Logger.ErrorContext(ctx, "failed to update configuration", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to update configuration"}
	}

	return cfg, nil
}

// preserveStoragePolicySecrets keeps previously stored storage credentials when
// an update omits them. GetConfiguration redacts these secrets, and the settings
// UI resubmits the whole storage policy on save, so a blank incoming secret means
// "unchanged", not "clear". Without this, saving any config field through the
// dashboard would wipe the stored S3/Azure/on-prem storage credentials. Secrets
// are only carried over within the same storage type, so switching type still
// applies the incoming values.
func preserveStoragePolicySecrets(next, prev *datastore.StoragePolicyConfiguration) {
	if next == nil || prev == nil {
		return
	}

	if next.S3 != nil && prev.S3 != nil {
		if next.S3.AccessKey.String == "" {
			next.S3.AccessKey = prev.S3.AccessKey
		}
		if next.S3.SecretKey.String == "" {
			next.S3.SecretKey = prev.S3.SecretKey
		}
		if next.S3.SessionToken.String == "" {
			next.S3.SessionToken = prev.S3.SessionToken
		}
	}

	if next.AzureBlob != nil && prev.AzureBlob != nil {
		if next.AzureBlob.AccountKey.String == "" {
			next.AzureBlob.AccountKey = prev.AzureBlob.AccountKey
		}
	}

	if next.OnPrem != nil && prev.OnPrem != nil {
		if next.OnPrem.Path.String == "" {
			next.OnPrem.Path = prev.OnPrem.Path
		}
	}
}
