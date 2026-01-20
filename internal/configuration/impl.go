package configuration

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/configuration/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

// Service implements the ConfigurationRepository using SQLc-generated queries
type Service struct {
	logger log.StdLogger
	repo   repo.Querier  // SQLc-generated interface
	db     *pgxpool.Pool // Connection pool
}

// Ensure Service implements datastore.ConfigurationRepository at compile time
var _ datastore.ConfigurationRepository = (*Service)(nil)

// New creates a new Configuration Service
func New(logger log.StdLogger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// boolToText converts bool to "true"/"false" string for database storage
func boolToText(b bool) string {
	return util.BoolToText(b)
}

// textToBool converts "true"/"false" string to bool
func textToBool(s string) bool {
	return s == "true"
}

// configurationToCreateParams converts Configuration to CreateConfigurationParams
func configurationToCreateParams(cfg *datastore.Configuration) repo.CreateConfigurationParams {
	params := repo.CreateConfigurationParams{
		ID:                 cfg.UID,
		IsAnalyticsEnabled: boolToText(cfg.IsAnalyticsEnabled),
		IsSignupEnabled:    cfg.IsSignupEnabled,
	}

	// Handle storage policy based on type
	if cfg.StoragePolicy != nil {
		params.StoragePolicyType = string(cfg.StoragePolicy.Type)

		if cfg.StoragePolicy.Type == datastore.OnPrem && cfg.StoragePolicy.OnPrem != nil {
			params.OnPremPath = common.NullStringToPgText(cfg.StoragePolicy.OnPrem.Path)
			// Set S3 fields to NULL
			params.S3Prefix = pgtype.Text{Valid: false}
			params.S3Bucket = pgtype.Text{Valid: false}
			params.S3AccessKey = pgtype.Text{Valid: false}
			params.S3SecretKey = pgtype.Text{Valid: false}
			params.S3Region = pgtype.Text{Valid: false}
			params.S3SessionToken = pgtype.Text{Valid: false}
			params.S3Endpoint = pgtype.Text{Valid: false}
		} else if cfg.StoragePolicy.S3 != nil { // S3
			params.S3Prefix = common.NullStringToPgText(cfg.StoragePolicy.S3.Prefix)
			params.S3Bucket = common.NullStringToPgText(cfg.StoragePolicy.S3.Bucket)
			params.S3AccessKey = common.NullStringToPgText(cfg.StoragePolicy.S3.AccessKey)
			params.S3SecretKey = common.NullStringToPgText(cfg.StoragePolicy.S3.SecretKey)
			params.S3Region = common.NullStringToPgText(cfg.StoragePolicy.S3.Region)
			params.S3SessionToken = common.NullStringToPgText(cfg.StoragePolicy.S3.SessionToken)
			params.S3Endpoint = common.NullStringToPgText(cfg.StoragePolicy.S3.Endpoint)
			// Set OnPrem to NULL
			params.OnPremPath = pgtype.Text{Valid: false}
		}
	}

	// Handle retention policy
	rc := cfg.GetRetentionPolicyConfig()
	params.RetentionPolicyPolicy = rc.Policy
	params.RetentionPolicyEnabled = rc.IsRetentionPolicyEnabled

	return params
}

// configurationToUpdateParams converts Configuration to UpdateConfigurationParams
func configurationToUpdateParams(cfg *datastore.Configuration) repo.UpdateConfigurationParams {
	params := repo.UpdateConfigurationParams{
		ID:                 cfg.UID,
		IsAnalyticsEnabled: boolToText(cfg.IsAnalyticsEnabled),
		IsSignupEnabled:    cfg.IsSignupEnabled,
	}

	// Handle storage policy based on type
	if cfg.StoragePolicy != nil {
		params.StoragePolicyType = string(cfg.StoragePolicy.Type)

		if cfg.StoragePolicy.Type == datastore.OnPrem && cfg.StoragePolicy.OnPrem != nil {
			params.OnPremPath = common.NullStringToPgText(cfg.StoragePolicy.OnPrem.Path)
			// Set S3 fields to NULL
			params.S3Prefix = pgtype.Text{Valid: false}
			params.S3Bucket = pgtype.Text{Valid: false}
			params.S3AccessKey = pgtype.Text{Valid: false}
			params.S3SecretKey = pgtype.Text{Valid: false}
			params.S3Region = pgtype.Text{Valid: false}
			params.S3SessionToken = pgtype.Text{Valid: false}
			params.S3Endpoint = pgtype.Text{Valid: false}
		} else if cfg.StoragePolicy.S3 != nil { // S3
			params.S3Prefix = common.NullStringToPgText(cfg.StoragePolicy.S3.Prefix)
			params.S3Bucket = common.NullStringToPgText(cfg.StoragePolicy.S3.Bucket)
			params.S3AccessKey = common.NullStringToPgText(cfg.StoragePolicy.S3.AccessKey)
			params.S3SecretKey = common.NullStringToPgText(cfg.StoragePolicy.S3.SecretKey)
			params.S3Region = common.NullStringToPgText(cfg.StoragePolicy.S3.Region)
			params.S3SessionToken = common.NullStringToPgText(cfg.StoragePolicy.S3.SessionToken)
			params.S3Endpoint = common.NullStringToPgText(cfg.StoragePolicy.S3.Endpoint)
			// Set OnPrem to NULL
			params.OnPremPath = pgtype.Text{Valid: false}
		}
	}

	// Handle retention policy
	rc := cfg.GetRetentionPolicyConfig()
	params.RetentionPolicyPolicy = rc.Policy
	params.RetentionPolicyEnabled = rc.IsRetentionPolicyEnabled

	return params
}

// rowToConfiguration converts LoadConfigurationRow to Configuration
func rowToConfiguration(row repo.LoadConfigurationRow) *datastore.Configuration {
	cfg := &datastore.Configuration{
		UID:                row.ID,
		IsAnalyticsEnabled: textToBool(row.IsAnalyticsEnabled),
		IsSignupEnabled:    row.IsSignupEnabled,
		CreatedAt:          row.CreatedAt.Time,
		UpdatedAt:          row.UpdatedAt.Time,
		DeletedAt:          common.PgTimestamptzToNullTime(row.DeletedAt),
	}

	// Reconstruct storage policy
	cfg.StoragePolicy = &datastore.StoragePolicyConfiguration{
		Type: datastore.StorageType(row.StoragePolicyType),
	}

	if row.StoragePolicyType == string(datastore.OnPrem) {
		cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
			Path: common.PgTextToNullString(row.OnPremPath),
		}
		// Create empty S3 storage to match legacy behavior
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
		cfg.StoragePolicy.S3 = &datastore.S3Storage{
			Prefix:       common.PgTextToNullString(row.S3Prefix),
			Bucket:       common.PgTextToNullString(row.S3Bucket),
			AccessKey:    common.PgTextToNullString(row.S3AccessKey),
			SecretKey:    common.PgTextToNullString(row.S3SecretKey),
			Region:       common.PgTextToNullString(row.S3Region),
			SessionToken: common.PgTextToNullString(row.S3SessionToken),
			Endpoint:     common.PgTextToNullString(row.S3Endpoint),
		}
		// Create empty OnPrem storage to match legacy behavior
		cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
			Path: null.NewString("", false),
		}
	}

	// Reconstruct retention policy
	cfg.RetentionPolicy = &datastore.RetentionPolicyConfiguration{
		Policy:                   row.RetentionPolicyPolicy,
		IsRetentionPolicyEnabled: row.RetentionPolicyEnabled,
	}

	return cfg
}

// ============================================================================
// Service Implementation
// ============================================================================

// CreateConfiguration creates a new configuration
func (s *Service) CreateConfiguration(ctx context.Context, cfg *datastore.Configuration) error {
	if cfg == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("configuration cannot be nil"))
	}

	// Normalize storage policy - ensure empty S3 fields for OnPrem and vice versa
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

	params := configurationToCreateParams(cfg)

	err := s.repo.CreateConfiguration(ctx, params)
	if err != nil {
		s.logger.WithError(err).Error("failed to create configuration")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	return nil
}

// LoadConfiguration loads the single configuration (should only be one)
func (s *Service) LoadConfiguration(ctx context.Context) (*datastore.Configuration, error) {
	row, err := s.repo.LoadConfiguration(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrConfigNotFound
		}
		s.logger.WithError(err).Error("failed to load configuration")
		return nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	cfg := rowToConfiguration(row)
	return cfg, nil
}

// UpdateConfiguration updates an existing configuration
func (s *Service) UpdateConfiguration(ctx context.Context, cfg *datastore.Configuration) error {
	if cfg == nil {
		return util.NewServiceError(http.StatusBadRequest, errors.New("configuration cannot be nil"))
	}

	// Normalize storage policy - ensure empty S3 fields for OnPrem and vice versa
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

	params := configurationToUpdateParams(cfg)

	result, err := s.repo.UpdateConfiguration(ctx, params)
	if err != nil {
		s.logger.WithError(err).Error("failed to update configuration")
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() == 0 {
		return util.NewServiceError(http.StatusNotFound, errors.New("configuration not found or not updated"))
	}

	return nil
}
