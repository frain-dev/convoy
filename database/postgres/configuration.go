package postgres

import (
	"context"
	"database/sql"
	"errors"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"gopkg.in/guregu/null.v4"
)

const (
	createConfiguration = `
	INSERT INTO convoy.configurations(
		id, is_analytics_enabled, is_signup_enabled,
		storage_policy_type, on_prem_path, s3_prefix,
		s3_bucket, s3_access_key, s3_secret_key,
		s3_region, s3_session_token, s3_endpoint,
		retention_policy_policy, retention_policy_enabled,
		cb_sample_rate,cb_error_timeout,
		cb_failure_threshold, cb_success_threshold,
		cb_observability_window,
		cb_consecutive_failure_threshold, cb_minimum_request_count
	  )
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21);
	`

	fetchConfiguration = `
	SELECT
		id,
		is_analytics_enabled,
		is_signup_enabled,
		retention_policy_enabled AS "retention_policy.enabled",
		retention_policy_policy AS "retention_policy.policy",
		storage_policy_type AS "storage_policy.type",
		on_prem_path AS "storage_policy.on_prem.path",
		s3_bucket AS "storage_policy.s3.bucket",
		s3_access_key AS "storage_policy.s3.access_key",
		s3_secret_key AS "storage_policy.s3.secret_key",
		s3_region AS "storage_policy.s3.region",
		s3_session_token AS "storage_policy.s3.session_token",
		s3_endpoint AS "storage_policy.s3.endpoint",
		s3_prefix AS "storage_policy.s3.prefix",
		cb_sample_rate AS "circuit_breaker.sample_rate",
		cb_error_timeout AS "circuit_breaker.error_timeout",
		cb_failure_threshold AS "circuit_breaker.failure_threshold",
		cb_success_threshold AS "circuit_breaker.success_threshold",
		cb_observability_window AS "circuit_breaker.observability_window",
		cb_minimum_request_count as "circuit_breaker.minimum_request_count",
		cb_consecutive_failure_threshold AS "circuit_breaker.consecutive_failure_threshold",
		created_at,
		updated_at,
		deleted_at
	FROM convoy.configurations
	WHERE deleted_at IS NULL LIMIT 1;
	`

	updateConfiguration = `
	UPDATE
		convoy.configurations
	SET
		is_analytics_enabled = $2,
		is_signup_enabled = $3,
		storage_policy_type = $4,
		on_prem_path = $5,
		s3_bucket = $6,
		s3_access_key = $7,
		s3_secret_key = $8,
		s3_region = $9,
		s3_session_token = $10,
		s3_endpoint = $11,
		s3_prefix = $12,
		retention_policy_policy = $13,
		retention_policy_enabled = $14,
		cb_sample_rate = $15,
		cb_error_timeout = $16,
		cb_failure_threshold = $17,
		cb_success_threshold = $18,
		cb_observability_window = $19,
		cb_consecutive_failure_threshold = $20,
		cb_minimum_request_count = $21,
		updated_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL;
	`
)

type configRepo struct {
	db database.Database
}

func NewConfigRepo(db database.Database) datastore.ConfigurationRepository {
	return &configRepo{db: db}
}

func (c *configRepo) CreateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	if config.StoragePolicy.Type == datastore.OnPrem {
		config.StoragePolicy.S3 = &datastore.S3Storage{
			Prefix:       null.NewString("", false),
			Bucket:       null.NewString("", false),
			AccessKey:    null.NewString("", false),
			SecretKey:    null.NewString("", false),
			Region:       null.NewString("", false),
			SessionToken: null.NewString("", false),
			Endpoint:     null.NewString("", false),
		}
	} else {
		config.StoragePolicy.OnPrem = &datastore.OnPremStorage{
			Path: null.NewString("", false),
		}
	}

	rc := config.GetRetentionPolicyConfig()
	cb := config.GetCircuitBreakerConfig()

	r, err := c.db.GetDB().ExecContext(ctx, createConfiguration,
		config.UID,
		util.BoolToText(config.IsAnalyticsEnabled),
		config.IsSignupEnabled,
		config.StoragePolicy.Type,
		config.StoragePolicy.OnPrem.Path,
		config.StoragePolicy.S3.Prefix,
		config.StoragePolicy.S3.Bucket,
		config.StoragePolicy.S3.AccessKey,
		config.StoragePolicy.S3.SecretKey,
		config.StoragePolicy.S3.Region,
		config.StoragePolicy.S3.SessionToken,
		config.StoragePolicy.S3.Endpoint,
		rc.Policy,
		rc.IsRetentionPolicyEnabled,
		cb.SampleRate,
		cb.ErrorTimeout,
		cb.FailureThreshold,
		cb.SuccessThreshold,
		cb.ObservabilityWindow,
		cb.ConsecutiveFailureThreshold,
		cb.MinimumRequestCount,
	)
	if err != nil {
		return err
	}

	nRows, err := r.RowsAffected()
	if err != nil {
		return err
	}

	if nRows < 1 {
		return errors.New("configuration not created")
	}

	return nil
}

func (c *configRepo) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	config := &datastore.Configuration{}
	err := c.db.GetReadDB().QueryRowxContext(ctx, fetchConfiguration).StructScan(config)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, datastore.ErrConfigNotFound
		}
		return nil, err
	}

	return config, nil
}

func (c *configRepo) UpdateConfiguration(ctx context.Context, cfg *datastore.Configuration) error {
	if cfg.StoragePolicy.Type == datastore.OnPrem {
		cfg.StoragePolicy.S3 = &datastore.S3Storage{
			Prefix:       null.NewString("", false),
			Bucket:       null.NewString("", false),
			AccessKey:    null.NewString("", false),
			SecretKey:    null.NewString("", false),
			Region:       null.NewString("", false),
			SessionToken: null.NewString("", false),
			Endpoint:     null.NewString("", false),
		}
	} else {
		cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
			Path: null.NewString("", false),
		}
	}

	rc := cfg.GetRetentionPolicyConfig()
	cb := cfg.GetCircuitBreakerConfig()

	result, err := c.db.GetDB().ExecContext(ctx, updateConfiguration,
		cfg.UID,
		util.BoolToText(cfg.IsAnalyticsEnabled),
		cfg.IsSignupEnabled,
		cfg.StoragePolicy.Type,
		cfg.StoragePolicy.OnPrem.Path,
		cfg.StoragePolicy.S3.Bucket,
		cfg.StoragePolicy.S3.AccessKey,
		cfg.StoragePolicy.S3.SecretKey,
		cfg.StoragePolicy.S3.Region,
		cfg.StoragePolicy.S3.SessionToken,
		cfg.StoragePolicy.S3.Endpoint,
		cfg.StoragePolicy.S3.Prefix,
		rc.Policy,
		rc.IsRetentionPolicyEnabled,
		cb.SampleRate,
		cb.ErrorTimeout,
		cb.FailureThreshold,
		cb.SuccessThreshold,
		cb.ObservabilityWindow,
		cb.ConsecutiveFailureThreshold,
		cb.MinimumRequestCount,
	)
	if err != nil {
		return err
	}

	nRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if nRows < 1 {
		return errors.New("configuration not updated")
	}

	return nil
}
