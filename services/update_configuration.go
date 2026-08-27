package services

import (
	"context"
	"errors"
	"strings"

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
		if err := assertStoragePolicyFields(cfg.StoragePolicy); err != nil {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}
	}

	if c.Config.RetentionPolicy != nil {
		next := c.Config.RetentionPolicy.Transform()
		if cfg.RetentionPolicy != nil {
			// Partial retention_policy payloads must not wipe the other knob.
			if c.Config.RetentionPolicy.Enabled == nil {
				next.Enabled = cfg.RetentionPolicy.Enabled
			}
			if next.Period == "" {
				next.Period = cfg.RetentionPolicy.Period
			}
		}
		cfg.RetentionPolicy = next
	}

	if c.Config.WebhookArchiving != nil {
		cfg.WebhookArchiving = c.Config.WebhookArchiving.Transform()
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
//
// When the type is unchanged but the nested backend object is nil (Admin form
// has no azure_blob fields today), keep the previous subtree wholesale.
func preserveStoragePolicySecrets(next, prev *datastore.StoragePolicyConfiguration) {
	if next == nil || prev == nil {
		return
	}

	if next.Type != prev.Type {
		return
	}

	switch next.Type {
	case datastore.S3:
		if next.S3 == nil {
			next.S3 = prev.S3
			return
		}
		if prev.S3 == nil {
			return
		}
		if next.S3.AccessKey.String == "" {
			next.S3.AccessKey = prev.S3.AccessKey
		}
		if next.S3.SecretKey.String == "" {
			next.S3.SecretKey = prev.S3.SecretKey
		}
		if next.S3.SessionToken.String == "" {
			next.S3.SessionToken = prev.S3.SessionToken
		}
		// Admin form has no endpoint/prefix controls; GET drops prefix on redact.
		// Blank means keep so MinIO and prefixed buckets survive a retention save.
		if next.S3.Endpoint.String == "" {
			next.S3.Endpoint = prev.S3.Endpoint
		}
		if next.S3.Prefix.String == "" {
			next.S3.Prefix = prev.S3.Prefix
		}
	case datastore.AzureBlob:
		if next.AzureBlob == nil {
			next.AzureBlob = prev.AzureBlob
			return
		}
		if prev.AzureBlob == nil {
			return
		}
		if next.AzureBlob.AccountKey.String == "" {
			next.AzureBlob.AccountKey = prev.AzureBlob.AccountKey
		}
		if next.AzureBlob.Endpoint.String == "" {
			next.AzureBlob.Endpoint = prev.AzureBlob.Endpoint
		}
		if next.AzureBlob.Prefix.String == "" {
			next.AzureBlob.Prefix = prev.AzureBlob.Prefix
		}
	case datastore.OnPrem:
		if next.OnPrem == nil {
			next.OnPrem = prev.OnPrem
			return
		}
		if prev.OnPrem == nil {
			return
		}
		if next.OnPrem.Path.String == "" {
			next.OnPrem.Path = prev.OnPrem.Path
		}
	}
}

// assertStoragePolicyFields rejects incomplete storage after blank-secret
// preserve. Unlike StoragePolicyUsable, /dev/null paths are allowed here so
// operators can still update retention/archiving while a sentinel path is set.
// Credential pairs are AND: half-filled S3/Azure is rejected.
func assertStoragePolicyFields(sp *datastore.StoragePolicyConfiguration) error {
	if sp == nil {
		return nil
	}

	switch sp.Type {
	case datastore.OnPrem:
		if sp.OnPrem == nil || strings.TrimSpace(sp.OnPrem.Path.ValueOrZero()) == "" {
			return errors.New("please provide an on_prem storage path")
		}
	case datastore.S3:
		if sp.S3 == nil || strings.TrimSpace(sp.S3.Bucket.ValueOrZero()) == "" {
			return errors.New("please provide a bucket name")
		}
		access := strings.TrimSpace(sp.S3.AccessKey.ValueOrZero())
		secret := strings.TrimSpace(sp.S3.SecretKey.ValueOrZero())
		if access == "" || secret == "" {
			return errors.New("please provide s3 access_key and secret_key")
		}
	case datastore.AzureBlob:
		if sp.AzureBlob == nil {
			return errors.New("please provide azure_blob storage configuration")
		}
		if strings.TrimSpace(sp.AzureBlob.AccountName.ValueOrZero()) == "" {
			return errors.New("please provide an azure account_name")
		}
		if strings.TrimSpace(sp.AzureBlob.AccountKey.ValueOrZero()) == "" {
			return errors.New("please provide an azure account_key")
		}
		if strings.TrimSpace(sp.AzureBlob.ContainerName.ValueOrZero()) == "" {
			return errors.New("please provide an azure container_name")
		}
	default:
		return errors.New("please provide a valid storage type")
	}

	return nil
}
