package configuration

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/configuration/repo"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/util"
)

// Service implements the ConfigurationRepository using SQLc-generated queries
type Service struct {
	logger log.Logger
	repo   repo.Querier  // SQLc-generated interface
	db     *pgxpool.Pool // Connection pool
}

// Ensure Service implements datastore.ConfigurationRepository at compile time
var _ datastore.ConfigurationRepository = (*Service)(nil)

// New creates a new Configuration Service
func New(logger log.Logger, db database.Database) *Service {
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
		ID:                 common.StringToPgText(cfg.UID),
		IsAnalyticsEnabled: common.StringToPgText(boolToText(cfg.IsAnalyticsEnabled)),
		IsSignupEnabled:    pgtype.Bool{Bool: cfg.IsSignupEnabled, Valid: true},
	}

	// Handle storage policy based on type
	if cfg.StoragePolicy != nil {
		params.StoragePolicyType = common.StringToPgText(string(cfg.StoragePolicy.Type))
		setStoragePolicyCreateParams(&params, cfg.StoragePolicy)
	}

	// Handle retention policy
	rc := cfg.GetRetentionPolicyConfig()
	params.RetentionPolicyPolicy = common.StringToPgText(rc.Policy)
	params.RetentionPolicyEnabled = pgtype.Bool{Bool: rc.IsRetentionPolicyEnabled, Valid: true}

	return params
}

// setStoragePolicyCreateParams populates storage fields on CreateConfigurationParams,
// setting unused backends to NULL.
func setStoragePolicyCreateParams(params *repo.CreateConfigurationParams, sp *datastore.StoragePolicyConfiguration) {
	nullText := pgtype.Text{Valid: false}

	// Default all to NULL
	params.OnPremPath = nullText
	params.S3Prefix = nullText
	params.S3Bucket = nullText
	params.S3AccessKey = nullText
	params.S3SecretKey = nullText
	params.S3Region = nullText
	params.S3SessionToken = nullText
	params.S3Endpoint = nullText
	params.AzureAccountName = nullText
	params.AzureAccountKey = nullText
	params.AzureContainerName = nullText
	params.AzureEndpoint = nullText
	params.AzurePrefix = nullText

	switch sp.Type {
	case datastore.OnPrem:
		if sp.OnPrem != nil {
			params.OnPremPath = common.NullStringToPgText(sp.OnPrem.Path)
		}
	case datastore.S3:
		if sp.S3 != nil {
			params.S3Prefix = common.NullStringToPgText(sp.S3.Prefix)
			params.S3Bucket = common.NullStringToPgText(sp.S3.Bucket)
			params.S3AccessKey = common.NullStringToPgText(sp.S3.AccessKey)
			params.S3SecretKey = common.NullStringToPgText(sp.S3.SecretKey)
			params.S3Region = common.NullStringToPgText(sp.S3.Region)
			params.S3SessionToken = common.NullStringToPgText(sp.S3.SessionToken)
			params.S3Endpoint = common.NullStringToPgText(sp.S3.Endpoint)
		}
	case datastore.AzureBlob:
		if sp.AzureBlob != nil {
			params.AzureAccountName = common.NullStringToPgText(sp.AzureBlob.AccountName)
			params.AzureAccountKey = common.NullStringToPgText(sp.AzureBlob.AccountKey)
			params.AzureContainerName = common.NullStringToPgText(sp.AzureBlob.ContainerName)
			params.AzureEndpoint = common.NullStringToPgText(sp.AzureBlob.Endpoint)
			params.AzurePrefix = common.NullStringToPgText(sp.AzureBlob.Prefix)
		}
	}
}

// configurationToUpdateParams converts Configuration to UpdateConfigurationParams
func configurationToUpdateParams(cfg *datastore.Configuration) repo.UpdateConfigurationParams {
	params := repo.UpdateConfigurationParams{
		ID:                 common.StringToPgText(cfg.UID),
		IsAnalyticsEnabled: common.StringToPgText(boolToText(cfg.IsAnalyticsEnabled)),
		IsSignupEnabled:    pgtype.Bool{Bool: cfg.IsSignupEnabled, Valid: true},
	}

	// Handle storage policy based on type
	if cfg.StoragePolicy != nil {
		params.StoragePolicyType = common.StringToPgText(string(cfg.StoragePolicy.Type))
		setStoragePolicyUpdateParams(&params, cfg.StoragePolicy)
	}

	// Handle retention policy
	rc := cfg.GetRetentionPolicyConfig()
	params.RetentionPolicyPolicy = common.StringToPgText(rc.Policy)
	params.RetentionPolicyEnabled = pgtype.Bool{Bool: rc.IsRetentionPolicyEnabled, Valid: true}

	return params
}

// setStoragePolicyUpdateParams populates storage fields on UpdateConfigurationParams,
// setting unused backends to NULL.
func setStoragePolicyUpdateParams(params *repo.UpdateConfigurationParams, sp *datastore.StoragePolicyConfiguration) {
	nullText := pgtype.Text{Valid: false}

	// Default all to NULL
	params.OnPremPath = nullText
	params.S3Prefix = nullText
	params.S3Bucket = nullText
	params.S3AccessKey = nullText
	params.S3SecretKey = nullText
	params.S3Region = nullText
	params.S3SessionToken = nullText
	params.S3Endpoint = nullText
	params.AzureAccountName = nullText
	params.AzureAccountKey = nullText
	params.AzureContainerName = nullText
	params.AzureEndpoint = nullText
	params.AzurePrefix = nullText

	switch sp.Type {
	case datastore.OnPrem:
		if sp.OnPrem != nil {
			params.OnPremPath = common.NullStringToPgText(sp.OnPrem.Path)
		}
	case datastore.S3:
		if sp.S3 != nil {
			params.S3Prefix = common.NullStringToPgText(sp.S3.Prefix)
			params.S3Bucket = common.NullStringToPgText(sp.S3.Bucket)
			params.S3AccessKey = common.NullStringToPgText(sp.S3.AccessKey)
			params.S3SecretKey = common.NullStringToPgText(sp.S3.SecretKey)
			params.S3Region = common.NullStringToPgText(sp.S3.Region)
			params.S3SessionToken = common.NullStringToPgText(sp.S3.SessionToken)
			params.S3Endpoint = common.NullStringToPgText(sp.S3.Endpoint)
		}
	case datastore.AzureBlob:
		if sp.AzureBlob != nil {
			params.AzureAccountName = common.NullStringToPgText(sp.AzureBlob.AccountName)
			params.AzureAccountKey = common.NullStringToPgText(sp.AzureBlob.AccountKey)
			params.AzureContainerName = common.NullStringToPgText(sp.AzureBlob.ContainerName)
			params.AzureEndpoint = common.NullStringToPgText(sp.AzureBlob.Endpoint)
			params.AzurePrefix = common.NullStringToPgText(sp.AzureBlob.Prefix)
		}
	}
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

	switch datastore.StorageType(row.StoragePolicyType) {
	case datastore.OnPrem:
		cfg.StoragePolicy.OnPrem = &datastore.OnPremStorage{
			Path: common.PgTextToNullString(row.OnPremPath),
		}
	case datastore.S3:
		cfg.StoragePolicy.S3 = &datastore.S3Storage{
			Prefix:       common.PgTextToNullString(row.S3Prefix),
			Bucket:       common.PgTextToNullString(row.S3Bucket),
			AccessKey:    common.PgTextToNullString(row.S3AccessKey),
			SecretKey:    common.PgTextToNullString(row.S3SecretKey),
			Region:       common.PgTextToNullString(row.S3Region),
			SessionToken: common.PgTextToNullString(row.S3SessionToken),
			Endpoint:     common.PgTextToNullString(row.S3Endpoint),
		}
	case datastore.AzureBlob:
		cfg.StoragePolicy.AzureBlob = &datastore.AzureBlobStorage{
			AccountName:   common.PgTextToNullString(row.AzureAccountName),
			AccountKey:    common.PgTextToNullString(row.AzureAccountKey),
			ContainerName: common.PgTextToNullString(row.AzureContainerName),
			Endpoint:      common.PgTextToNullString(row.AzureEndpoint),
			Prefix:        common.PgTextToNullString(row.AzurePrefix),
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

	params := configurationToCreateParams(cfg)

	err := s.repo.CreateConfiguration(ctx, params)
	if err != nil {
		s.logger.Error("failed to create configuration", "error", err)
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
		s.logger.Error("failed to load configuration", "error", err)
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

	params := configurationToUpdateParams(cfg)

	result, err := s.repo.UpdateConfiguration(ctx, params)
	if err != nil {
		s.logger.Error("failed to update configuration", "error", err)
		return util.NewServiceError(http.StatusInternalServerError, err)
	}

	if result.RowsAffected() == 0 {
		return util.NewServiceError(http.StatusNotFound, errors.New("configuration not found or not updated"))
	}

	return nil
}
