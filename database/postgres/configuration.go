package postgres

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"
)

const (
	createConfiguration = `
	-- configuration.go:createConfiguration
	INSERT INTO convoy.configurations(
		id, is_analytics_enabled, is_signup_enabled, 
		storage_policy_type, on_prem_path, 
		s3_bucket, s3_access_key, s3_secret_key, 
		s3_region, s3_session_token, s3_endpoint
	  ) 
	  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
	`

	fetchConfiguration = `
	-- project.go:fetchConfiguration
	SELECT 
		id,
		is_analytics_enabled,
		is_signup_enabled,
		storage_policy_type AS "storage_policy.type",
		on_prem_path AS "storage_policy.on_prem.path",
		s3_bucket AS "storage_policy.s3.bucket",
		s3_access_key AS "storage_policy.s3.access_key",
		s3_secret_key AS "storage_policy.s3.secret_key",
		s3_region AS "storage_policy.s3.region",
		s3_session_token AS "storage_policy.s3.session_token",
		s3_endpoint AS "storage_policy.s3.endpoint",
		created_at,
		updated_at,
		deleted_at
	FROM convoy.configurations
	WHERE id = 'default';
	`

	updateConfiguration = `
	-- configuration.go:updateConfiguration
	UPDATE
		convoy.configurations
	SET
		is_analytics_enabled = $1,
		is_signup_enabled = $2,
		storage_policy_type = $3,
		on_prem_path = $4,
		s3_bucket = $5,
		s3_access_key = $6,
		s3_secret_key = $7,
		s3_region = $8,
		s3_session_token = $9, 
		s3_endpoint = $10,
		updated_at = now()
	WHERE id = 'default';
	`
)

type configRepo struct {
	db *sqlx.DB
}

func NewConfigRepo(db *sqlx.DB) datastore.ConfigurationRepository {
	return &configRepo{db: db}
}

func (c *configRepo) CreateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	if config.StoragePolicy.Type == datastore.OnPrem {
		config.StoragePolicy.S3 = &datastore.S3Storage{
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

	cResult, err := c.db.Exec(createConfiguration,
		config.UID,
		config.IsAnalyticsEnabled,
		config.IsSignupEnabled,
		config.StoragePolicy.Type,
		config.StoragePolicy.OnPrem.Path,
		config.StoragePolicy.S3.Bucket,
		config.StoragePolicy.S3.AccessKey,
		config.StoragePolicy.S3.SecretKey,
		config.StoragePolicy.S3.Region,
		config.StoragePolicy.S3.SessionToken,
		config.StoragePolicy.S3.Endpoint,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := cResult.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectNotCreated
	}

	return nil
}

func (c *configRepo) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	var config datastore.Configuration
	err := c.db.Get(&config, fetchConfiguration)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *configRepo) UpdateConfiguration(ctx context.Context, config *datastore.Configuration) error {
	if config.StoragePolicy.Type == datastore.OnPrem {
		config.StoragePolicy.S3 = &datastore.S3Storage{
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

	result, err := c.db.Exec(updateConfiguration,
		config.IsAnalyticsEnabled,
		config.IsSignupEnabled,
		config.StoragePolicy.Type,
		config.StoragePolicy.OnPrem.Path,
		config.StoragePolicy.S3.Bucket,
		config.StoragePolicy.S3.AccessKey,
		config.StoragePolicy.S3.SecretKey,
		config.StoragePolicy.S3.Region,
		config.StoragePolicy.S3.SessionToken,
		config.StoragePolicy.S3.Endpoint,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrProjectNotUpdated
	}

	return nil
}
