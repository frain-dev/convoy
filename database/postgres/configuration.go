package postgres

import (
	"context"
	"errors"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jmoiron/sqlx"
	"gopkg.in/guregu/null.v4"
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

	query := sq.Insert("convoy.configurations").
		Columns("id", "is_analytics_enabled",
			"is_signup_enabled", "storage_policy_type",
			"on_prem_path", "s3_bucket", "s3_access_key",
			"s3_secret_key", "s3_region",
			"s3_session_token", "s3_endpoint").
		Values(config.UID,
			config.IsAnalyticsEnabled,
			config.IsSignupEnabled,
			config.StoragePolicy.Type,
			config.StoragePolicy.OnPrem.Path,
			config.StoragePolicy.S3.Bucket,
			config.StoragePolicy.S3.AccessKey,
			config.StoragePolicy.S3.SecretKey,
			config.StoragePolicy.S3.Region,
			config.StoragePolicy.S3.SessionToken,
			config.StoragePolicy.S3.Endpoint).
		Suffix("RETURNING id").
		PlaceholderFormat(sq.Dollar)

	sql, vals, err := query.ToSql()
	if err != nil {
		return err
	}

	result, err := c.db.ExecContext(ctx, sql, vals...)
	if err != nil {
		return err
	}

	nRows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if nRows < 1 {
		return errors.New("configuration not created")
	}

	return nil
}

func (c *configRepo) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	query := sq.Select(
		"id",
		"is_analytics_enabled",
		"is_signup_enabled",
		`storage_policy_type AS "storage_policy.type"`,
		`on_prem_path AS "storage_policy.on_prem.path"`,
		`s3_bucket AS "storage_policy.s3.bucket"`,
		`s3_access_key AS "storage_policy.s3.access_key"`,
		`s3_secret_key AS "storage_policy.s3.secret_key"`,
		`s3_region AS "storage_policy.s3.region"`,
		`s3_session_token AS "storage_policy.s3.session_token"`,
		`s3_endpoint AS "storage_policy.s3.endpoint"`,
	).From("convoy.configurations").Where("id = 'default'")

	sql, _, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var config datastore.Configuration
	err = c.db.GetContext(ctx, &config, sql)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *configRepo) UpdateConfiguration(ctx context.Context, cfg *datastore.Configuration) error {
	if cfg.StoragePolicy.Type == datastore.OnPrem {
		cfg.StoragePolicy.S3 = &datastore.S3Storage{
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

	query := sq.Update("convoy.configurations").
		Set("is_analytics_enabled", cfg.IsAnalyticsEnabled).
		Set("is_signup_enabled", cfg.IsSignupEnabled).
		Set("storage_policy_type", cfg.StoragePolicy.Type).
		Set("on_prem_path", cfg.StoragePolicy.OnPrem.Path).
		Set("s3_bucket", cfg.StoragePolicy.S3.Bucket).
		Set("s3_access_key", cfg.StoragePolicy.S3.AccessKey).
		Set("s3_secret_key", cfg.StoragePolicy.S3.SecretKey).
		Set("s3_region", cfg.StoragePolicy.S3.Region).
		Set("s3_session_token", cfg.StoragePolicy.S3.SessionToken).
		Set("s3_endpoint", cfg.StoragePolicy.S3.Endpoint).
		Set("updated_at", time.Now()).
		Where("id = 'default'").
		PlaceholderFormat(sq.Dollar)

	sql, vals, err := query.ToSql()
	if err != nil {
		return err
	}

	result, err := c.db.ExecContext(ctx, sql, vals...)
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
