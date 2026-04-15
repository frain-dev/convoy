package feature_flags

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/feature_flags/repo"
	fflag "github.com/frain-dev/convoy/internal/pkg/fflag"
	log "github.com/frain-dev/convoy/pkg/logger"
)

// Service implements feature flag operations using sqlc-generated queries.
// It satisfies both fflag.FeatureFlagFetcher and fflag.EarlyAdopterFeatureFetcher.
type Service struct {
	logger log.Logger
	repo   repo.Querier
	db     *pgxpool.Pool
}

// Compile-time interface checks
var _ fflag.FeatureFlagFetcher = (*Service)(nil)
var _ fflag.EarlyAdopterFeatureFetcher = (*Service)(nil)

// New creates a new feature flags Service.
func New(logger log.Logger, db database.Database) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db.GetConn()),
		db:     db.GetConn(),
	}
}

// ============================================================================
// Row conversion helpers
// ============================================================================

func rowToFeatureFlag(row repo.FetchFeatureFlagByKeyRow) datastore.FeatureFlag {
	return datastore.FeatureFlag{
		UID:        row.ID,
		FeatureKey: row.FeatureKey,
		Enabled:    row.Enabled,
		CreatedAt:  common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:  common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

func rowToFeatureFlagByID(row repo.FetchFeatureFlagByIDRow) datastore.FeatureFlag {
	return datastore.FeatureFlag{
		UID:        row.ID,
		FeatureKey: row.FeatureKey,
		Enabled:    row.Enabled,
		CreatedAt:  common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:  common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

func loadRowToFeatureFlag(row repo.LoadFeatureFlagsRow) datastore.FeatureFlag {
	return datastore.FeatureFlag{
		UID:        row.ID,
		FeatureKey: row.FeatureKey,
		Enabled:    row.Enabled,
		CreatedAt:  common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:  common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

func rowToFeatureFlagOverride(row repo.FetchFeatureFlagOverrideRow) datastore.FeatureFlagOverride {
	return datastore.FeatureFlagOverride{
		UID:           row.ID,
		FeatureFlagID: row.FeatureFlagID,
		OwnerType:     row.OwnerType,
		OwnerID:       row.OwnerID,
		Enabled:       row.Enabled,
		EnabledAt:     common.PgTimestamptzToNullTime(row.EnabledAt),
		EnabledBy:     common.PgTextToNullString(row.EnabledBy),
		CreatedAt:     common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:     common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

func loadRowToFeatureFlagOverrideByOwner(row repo.LoadFeatureFlagOverridesByOwnerRow) datastore.FeatureFlagOverride {
	return datastore.FeatureFlagOverride{
		UID:           row.ID,
		FeatureFlagID: row.FeatureFlagID,
		OwnerType:     row.OwnerType,
		OwnerID:       row.OwnerID,
		Enabled:       row.Enabled,
		EnabledAt:     common.PgTimestamptzToNullTime(row.EnabledAt),
		EnabledBy:     common.PgTextToNullString(row.EnabledBy),
		CreatedAt:     common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:     common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

func loadRowToFeatureFlagOverrideByFF(row repo.LoadFeatureFlagOverridesByFeatureFlagRow) datastore.FeatureFlagOverride {
	return datastore.FeatureFlagOverride{
		UID:           row.ID,
		FeatureFlagID: row.FeatureFlagID,
		OwnerType:     row.OwnerType,
		OwnerID:       row.OwnerID,
		Enabled:       row.Enabled,
		EnabledAt:     common.PgTimestamptzToNullTime(row.EnabledAt),
		EnabledBy:     common.PgTextToNullString(row.EnabledBy),
		CreatedAt:     common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:     common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

func rowToEarlyAdopterFeature(row repo.FetchEarlyAdopterFeatureRow) datastore.EarlyAdopterFeature {
	return datastore.EarlyAdopterFeature{
		UID:            row.ID,
		OrganisationID: row.OrganisationID,
		FeatureKey:     row.FeatureKey,
		Enabled:        row.Enabled,
		EnabledBy:      common.PgTextToNullString(row.EnabledBy),
		EnabledAt:      common.PgTimestamptzToNullTime(row.EnabledAt),
		CreatedAt:      common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:      common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

func loadRowToEarlyAdopterFeature(row repo.LoadEarlyAdopterFeaturesByOrgRow) datastore.EarlyAdopterFeature {
	return datastore.EarlyAdopterFeature{
		UID:            row.ID,
		OrganisationID: row.OrganisationID,
		FeatureKey:     row.FeatureKey,
		Enabled:        row.Enabled,
		EnabledBy:      common.PgTextToNullString(row.EnabledBy),
		EnabledAt:      common.PgTimestamptzToNullTime(row.EnabledAt),
		CreatedAt:      common.PgTimestamptzToTime(row.CreatedAt),
		UpdatedAt:      common.PgTimestamptzToTime(row.UpdatedAt),
	}
}

// ============================================================================
// CRUD Methods — Feature Flags
// ============================================================================

// FetchFeatureFlagByKey fetches a feature flag by its key.
func (s *Service) FetchFeatureFlagByKey(ctx context.Context, key string) (*datastore.FeatureFlag, error) {
	row, err := s.repo.FetchFeatureFlagByKey(ctx, common.StringToPgText(key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrFeatureFlagNotFound
		}
		s.logger.Error("failed to fetch feature flag by key", "error", err)
		return nil, err
	}

	flag := rowToFeatureFlag(row)
	return &flag, nil
}

// FetchFeatureFlagByID fetches a feature flag by its ID.
func (s *Service) FetchFeatureFlagByID(ctx context.Context, id string) (*datastore.FeatureFlag, error) {
	row, err := s.repo.FetchFeatureFlagByID(ctx, common.StringToPgText(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrFeatureFlagNotFound
		}
		s.logger.Error("failed to fetch feature flag by id", "error", err)
		return nil, err
	}

	flag := rowToFeatureFlagByID(row)
	return &flag, nil
}

// LoadFeatureFlags fetches all feature flags ordered by key.
func (s *Service) LoadFeatureFlags(ctx context.Context) ([]datastore.FeatureFlag, error) {
	rows, err := s.repo.LoadFeatureFlags(ctx)
	if err != nil {
		s.logger.Error("failed to load feature flags", "error", err)
		return nil, err
	}

	flags := make([]datastore.FeatureFlag, 0, len(rows))
	for _, row := range rows {
		flags = append(flags, loadRowToFeatureFlag(row))
	}
	return flags, nil
}

// UpdateFeatureFlag updates the enabled state of a feature flag.
func (s *Service) UpdateFeatureFlag(ctx context.Context, featureFlagID string, enabled bool) error {
	err := s.repo.UpdateFeatureFlag(ctx, repo.UpdateFeatureFlagParams{
		ID:      common.StringToPgText(featureFlagID),
		Enabled: enabled,
	})
	if err != nil {
		s.logger.Error("failed to update feature flag", "error", err)
		return err
	}
	return nil
}

// ============================================================================
// CRUD Methods — Feature Flag Overrides
// ============================================================================

// FetchFeatureFlagOverrideByOwner fetches a feature flag override for a specific owner.
func (s *Service) FetchFeatureFlagOverrideByOwner(ctx context.Context, ownerType, ownerID, featureFlagID string) (*datastore.FeatureFlagOverride, error) {
	row, err := s.repo.FetchFeatureFlagOverride(ctx, repo.FetchFeatureFlagOverrideParams{
		OwnerType:     common.StringToPgText(ownerType),
		OwnerID:       common.StringToPgText(ownerID),
		FeatureFlagID: common.StringToPgText(featureFlagID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrFeatureFlagOverrideNotFound
		}
		s.logger.Error("failed to fetch feature flag override", "error", err)
		return nil, err
	}

	override := rowToFeatureFlagOverride(row)
	return &override, nil
}

// LoadFeatureFlagOverridesByOwner fetches all overrides for a specific owner.
func (s *Service) LoadFeatureFlagOverridesByOwner(ctx context.Context, ownerType, ownerID string) ([]datastore.FeatureFlagOverride, error) {
	rows, err := s.repo.LoadFeatureFlagOverridesByOwner(ctx, repo.LoadFeatureFlagOverridesByOwnerParams{
		OwnerType: common.StringToPgText(ownerType),
		OwnerID:   common.StringToPgText(ownerID),
	})
	if err != nil {
		s.logger.Error("failed to load feature flag overrides by owner", "error", err)
		return nil, err
	}

	overrides := make([]datastore.FeatureFlagOverride, 0, len(rows))
	for _, row := range rows {
		overrides = append(overrides, loadRowToFeatureFlagOverrideByOwner(row))
	}
	return overrides, nil
}

// LoadFeatureFlagOverridesByFeatureFlag fetches all overrides for a specific feature flag.
func (s *Service) LoadFeatureFlagOverridesByFeatureFlag(ctx context.Context, featureFlagID string) ([]datastore.FeatureFlagOverride, error) {
	rows, err := s.repo.LoadFeatureFlagOverridesByFeatureFlag(ctx, common.StringToPgText(featureFlagID))
	if err != nil {
		s.logger.Error("failed to load feature flag overrides by feature flag", "error", err)
		return nil, err
	}

	overrides := make([]datastore.FeatureFlagOverride, 0, len(rows))
	for _, row := range rows {
		overrides = append(overrides, loadRowToFeatureFlagOverrideByFF(row))
	}
	return overrides, nil
}

// UpsertFeatureFlagOverride creates or updates a feature flag override.
func (s *Service) UpsertFeatureFlagOverride(ctx context.Context, override *datastore.FeatureFlagOverride) error {
	if override.UID == "" {
		override.UID = ulid.Make().String()
	}

	// Handle nullable enabledAt
	enabledAt := common.NullTimeToPgTimestamptz(override.EnabledAt)
	if !override.EnabledAt.Valid && override.Enabled {
		enabledAt = common.TimeToPgTimestamptz(time.Now())
	}

	err := s.repo.UpsertFeatureFlagOverride(ctx, repo.UpsertFeatureFlagOverrideParams{
		ID:            common.StringToPgText(override.UID),
		FeatureFlagID: common.StringToPgText(override.FeatureFlagID),
		OwnerType:     common.StringToPgText(override.OwnerType),
		OwnerID:       common.StringToPgText(override.OwnerID),
		Enabled:       override.Enabled,
		EnabledAt:     enabledAt,
		EnabledBy:     common.NullStringToPgText(override.EnabledBy),
	})
	if err != nil {
		s.logger.Error("failed to upsert feature flag override", "error", err)
		return err
	}
	return nil
}

// DeleteFeatureFlagOverride deletes a feature flag override.
func (s *Service) DeleteFeatureFlagOverride(ctx context.Context, ownerType, ownerID, featureFlagID string) error {
	err := s.repo.DeleteFeatureFlagOverride(ctx, repo.DeleteFeatureFlagOverrideParams{
		OwnerType:     common.StringToPgText(ownerType),
		OwnerID:       common.StringToPgText(ownerID),
		FeatureFlagID: common.StringToPgText(featureFlagID),
	})
	if err != nil {
		s.logger.Error("failed to delete feature flag override", "error", err)
		return err
	}
	return nil
}

// ============================================================================
// CRUD Methods — Early Adopter Features
// ============================================================================

// GetEarlyAdopterFeature fetches an early adopter feature for an organisation.
// Named GetEarlyAdopterFeature to avoid conflict with the fflag.EarlyAdopterFeatureFetcher
// interface method FetchEarlyAdopterFeature.
func (s *Service) GetEarlyAdopterFeature(ctx context.Context, orgID, featureKey string) (*datastore.EarlyAdopterFeature, error) {
	row, err := s.repo.FetchEarlyAdopterFeature(ctx, repo.FetchEarlyAdopterFeatureParams{
		OrganisationID: common.StringToPgText(orgID),
		FeatureKey:     common.StringToPgText(featureKey),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, datastore.ErrEarlyAdopterFeatureNotFound
		}
		s.logger.Error("failed to fetch early adopter feature", "error", err)
		return nil, err
	}

	feature := rowToEarlyAdopterFeature(row)
	return &feature, nil
}

// LoadEarlyAdopterFeaturesByOrg fetches all early adopter features for an organisation.
func (s *Service) LoadEarlyAdopterFeaturesByOrg(ctx context.Context, orgID string) ([]datastore.EarlyAdopterFeature, error) {
	rows, err := s.repo.LoadEarlyAdopterFeaturesByOrg(ctx, common.StringToPgText(orgID))
	if err != nil {
		s.logger.Error("failed to load early adopter features by org", "error", err)
		return nil, err
	}

	features := make([]datastore.EarlyAdopterFeature, 0, len(rows))
	for _, row := range rows {
		features = append(features, loadRowToEarlyAdopterFeature(row))
	}
	return features, nil
}

// UpsertEarlyAdopterFeature creates or updates an early adopter feature.
func (s *Service) UpsertEarlyAdopterFeature(ctx context.Context, feature *datastore.EarlyAdopterFeature) error {
	if feature.UID == "" {
		feature.UID = ulid.Make().String()
	}

	// Handle nullable enabledAt
	enabledAt := common.NullTimeToPgTimestamptz(feature.EnabledAt)
	if !feature.EnabledAt.Valid && feature.Enabled {
		enabledAt = common.TimeToPgTimestamptz(time.Now())
	}

	err := s.repo.UpsertEarlyAdopterFeature(ctx, repo.UpsertEarlyAdopterFeatureParams{
		ID:             common.StringToPgText(feature.UID),
		OrganisationID: common.StringToPgText(feature.OrganisationID),
		FeatureKey:     common.StringToPgText(feature.FeatureKey),
		Enabled:        feature.Enabled,
		EnabledBy:      common.NullStringToPgText(feature.EnabledBy),
		EnabledAt:      enabledAt,
	})
	if err != nil {
		s.logger.Error("failed to upsert early adopter feature", "error", err)
		return err
	}
	return nil
}

// DeleteEarlyAdopterFeature deletes an early adopter feature.
func (s *Service) DeleteEarlyAdopterFeature(ctx context.Context, orgID, featureKey string) error {
	err := s.repo.DeleteEarlyAdopterFeature(ctx, repo.DeleteEarlyAdopterFeatureParams{
		OrganisationID: common.StringToPgText(orgID),
		FeatureKey:     common.StringToPgText(featureKey),
	})
	if err != nil {
		s.logger.Error("failed to delete early adopter feature", "error", err)
		return err
	}
	return nil
}

// ============================================================================
// Interface Methods — fflag.FeatureFlagFetcher
// ============================================================================

// FetchFeatureFlag implements fflag.FeatureFlagFetcher.
// It fetches a feature flag by key and returns the info needed by the fflag package.
func (s *Service) FetchFeatureFlag(ctx context.Context, key string) (*fflag.FeatureFlagInfo, error) {
	flag, err := s.FetchFeatureFlagByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	return &fflag.FeatureFlagInfo{
		UID:     flag.UID,
		Enabled: flag.Enabled,
	}, nil
}

// FetchFeatureFlagOverride implements fflag.FeatureFlagFetcher.
// It fetches a feature flag override and returns the info needed by the fflag package.
func (s *Service) FetchFeatureFlagOverride(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error) {
	override, err := s.FetchFeatureFlagOverrideByOwner(ctx, ownerType, ownerID, featureFlagID)
	if err != nil {
		return nil, err
	}

	return &fflag.FeatureFlagOverrideInfo{
		Enabled: override.Enabled,
	}, nil
}

// ============================================================================
// Interface Methods — fflag.EarlyAdopterFeatureFetcher
// ============================================================================

// FetchEarlyAdopterFeature implements fflag.EarlyAdopterFeatureFetcher.
// It fetches an early adopter feature and returns the info needed by the fflag package.
func (s *Service) FetchEarlyAdopterFeature(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
	feature, err := s.GetEarlyAdopterFeature(ctx, orgID, featureKey)
	if err != nil {
		return nil, err
	}

	return &fflag.EarlyAdopterFeatureInfo{
		Enabled: feature.Enabled,
	}, nil
}
