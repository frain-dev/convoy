package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

var (
	ErrFeatureFlagNotFound         = errors.New("feature flag not found")
	ErrFeatureFlagOverrideNotFound = errors.New("feature flag override not found")
	ErrEarlyAdopterFeatureNotFound = errors.New("early adopter feature not found")
)

const (
	fetchFeatureFlagByKey = `
	SELECT * FROM convoy.feature_flags
	WHERE feature_key = $1;
	`

	fetchFeatureFlagByID = `
	SELECT * FROM convoy.feature_flags
	WHERE id = $1;
	`

	loadFeatureFlags = `
	SELECT * FROM convoy.feature_flags
	ORDER BY feature_key;
	`

	fetchFeatureFlagOverride = `
	SELECT * FROM convoy.feature_flag_overrides
	WHERE owner_type = $1 AND owner_id = $2 AND feature_flag_id = $3;
	`

	loadFeatureFlagOverridesByOwner = `
	SELECT * FROM convoy.feature_flag_overrides
	WHERE owner_type = $1 AND owner_id = $2;
	`

	loadFeatureFlagOverridesByFeatureFlag = `
	SELECT * FROM convoy.feature_flag_overrides
	WHERE feature_flag_id = $1;
	`
)

// FetchFeatureFlagByKey fetches a feature flag by its key
func FetchFeatureFlagByKey(ctx context.Context, db database.Database, key string) (*datastore.FeatureFlag, error) {
	flag := &datastore.FeatureFlag{}
	err := db.GetDB().QueryRowxContext(ctx, fetchFeatureFlagByKey, key).StructScan(flag)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFeatureFlagNotFound
		}
		return nil, err
	}

	return flag, nil
}

// FetchFeatureFlagByID fetches a feature flag by its ID
func FetchFeatureFlagByID(ctx context.Context, db database.Database, id string) (*datastore.FeatureFlag, error) {
	flag := &datastore.FeatureFlag{}
	err := db.GetDB().QueryRowxContext(ctx, fetchFeatureFlagByID, id).StructScan(flag)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFeatureFlagNotFound
		}
		return nil, err
	}

	return flag, nil
}

// LoadFeatureFlags fetches all feature flags
func LoadFeatureFlags(ctx context.Context, db database.Database) ([]datastore.FeatureFlag, error) {
	flags := []datastore.FeatureFlag{}
	err := db.GetDB().SelectContext(ctx, &flags, loadFeatureFlags)
	if err != nil {
		return nil, err
	}

	return flags, nil
}

// FetchFeatureFlagOverride fetches a feature flag override for a specific owner
func FetchFeatureFlagOverride(ctx context.Context, db database.Database, ownerType, ownerID, featureFlagID string) (*datastore.FeatureFlagOverride, error) {
	override := &datastore.FeatureFlagOverride{}
	err := db.GetDB().QueryRowxContext(ctx, fetchFeatureFlagOverride, ownerType, ownerID, featureFlagID).StructScan(override)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFeatureFlagOverrideNotFound
		}
		return nil, err
	}

	return override, nil
}

// LoadFeatureFlagOverridesByOwner fetches all feature flag overrides for a specific owner
func LoadFeatureFlagOverridesByOwner(ctx context.Context, db database.Database, ownerType, ownerID string) ([]datastore.FeatureFlagOverride, error) {
	overrides := []datastore.FeatureFlagOverride{}
	err := db.GetDB().SelectContext(ctx, &overrides, loadFeatureFlagOverridesByOwner, ownerType, ownerID)
	if err != nil {
		return nil, err
	}

	return overrides, nil
}

// LoadFeatureFlagOverridesByFeatureFlag fetches all overrides for a specific feature flag
func LoadFeatureFlagOverridesByFeatureFlag(ctx context.Context, db database.Database, featureFlagID string) ([]datastore.FeatureFlagOverride, error) {
	overrides := []datastore.FeatureFlagOverride{}
	err := db.GetDB().SelectContext(ctx, &overrides, loadFeatureFlagOverridesByFeatureFlag, featureFlagID)
	if err != nil {
		return nil, err
	}

	return overrides, nil
}

const (
	createFeatureFlagOverride = `
	INSERT INTO convoy.feature_flag_overrides (id, feature_flag_id, owner_type, owner_id, enabled, enabled_at, enabled_by)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	ON CONFLICT (owner_type, owner_id, feature_flag_id) 
	DO UPDATE SET enabled = $5, enabled_at = $6, enabled_by = $7, updated_at = NOW();
	`

	deleteFeatureFlagOverride = `
	DELETE FROM convoy.feature_flag_overrides
	WHERE owner_type = $1 AND owner_id = $2 AND feature_flag_id = $3;
	`

	updateFeatureFlag = `
	UPDATE convoy.feature_flags
	SET enabled = $1, updated_at = NOW()
	WHERE id = $2;
	`
)

const (
	fetchEarlyAdopterFeature = `
	SELECT * FROM convoy.early_adopter_features
	WHERE organisation_id = $1 AND feature_key = $2;
	`

	loadEarlyAdopterFeaturesByOrg = `
	SELECT * FROM convoy.early_adopter_features
	WHERE organisation_id = $1
	ORDER BY feature_key;
	`

	upsertEarlyAdopterFeature = `
	INSERT INTO convoy.early_adopter_features (id, organisation_id, feature_key, enabled, enabled_by, enabled_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (organisation_id, feature_key) 
	DO UPDATE SET enabled = $4, enabled_by = $5, enabled_at = $6, updated_at = NOW();
	`

	deleteEarlyAdopterFeature = `
	DELETE FROM convoy.early_adopter_features
	WHERE organisation_id = $1 AND feature_key = $2;
	`
)

// UpsertFeatureFlagOverride creates or updates a feature flag override
func UpsertFeatureFlagOverride(ctx context.Context, db database.Database, override *datastore.FeatureFlagOverride) error {
	if override.UID == "" {
		override.UID = ulid.Make().String()
	}

	var enabledAt interface{}
	if override.EnabledAt.Valid {
		enabledAt = override.EnabledAt.Time
	} else if override.Enabled {
		enabledAt = time.Now()
	} else {
		enabledAt = nil
	}

	var enabledBy interface{}
	if override.EnabledBy.Valid {
		enabledBy = override.EnabledBy.String
	} else {
		enabledBy = nil
	}

	_, err := db.GetDB().ExecContext(ctx, createFeatureFlagOverride,
		override.UID, override.FeatureFlagID, override.OwnerType, override.OwnerID,
		override.Enabled, enabledAt, enabledBy)
	if err != nil {
		return err
	}

	return nil
}

// DeleteFeatureFlagOverride deletes a feature flag override
func DeleteFeatureFlagOverride(ctx context.Context, db database.Database, ownerType, ownerID, featureFlagID string) error {
	_, err := db.GetDB().ExecContext(ctx, deleteFeatureFlagOverride, ownerType, ownerID, featureFlagID)
	return err
}

// UpdateFeatureFlag updates the enabled state of a feature flag
func UpdateFeatureFlag(ctx context.Context, db database.Database, featureFlagID string, enabled bool) error {
	_, err := db.GetDB().ExecContext(ctx, updateFeatureFlag, enabled, featureFlagID)
	return err
}

// FetchEarlyAdopterFeature fetches an early adopter feature for an organisation
func FetchEarlyAdopterFeature(ctx context.Context, db database.Database, orgID, featureKey string) (*datastore.EarlyAdopterFeature, error) {
	feature := &datastore.EarlyAdopterFeature{}
	err := db.GetDB().QueryRowxContext(ctx, fetchEarlyAdopterFeature, orgID, featureKey).StructScan(feature)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEarlyAdopterFeatureNotFound
		}
		return nil, err
	}

	return feature, nil
}

// LoadEarlyAdopterFeaturesByOrg fetches all early adopter features for an organisation
func LoadEarlyAdopterFeaturesByOrg(ctx context.Context, db database.Database, orgID string) ([]datastore.EarlyAdopterFeature, error) {
	features := []datastore.EarlyAdopterFeature{}
	err := db.GetDB().SelectContext(ctx, &features, loadEarlyAdopterFeaturesByOrg, orgID)
	if err != nil {
		return nil, err
	}

	return features, nil
}

// UpsertEarlyAdopterFeature creates or updates an early adopter feature
func UpsertEarlyAdopterFeature(ctx context.Context, db database.Database, feature *datastore.EarlyAdopterFeature) error {
	if feature.UID == "" {
		feature.UID = ulid.Make().String()
	}

	var enabledAt interface{}
	if feature.EnabledAt.Valid {
		enabledAt = feature.EnabledAt.Time
	} else if feature.Enabled {
		enabledAt = time.Now()
	} else {
		enabledAt = nil
	}

	var enabledBy interface{}
	if feature.EnabledBy.Valid {
		enabledBy = feature.EnabledBy.String
	} else {
		enabledBy = nil
	}

	_, err := db.GetDB().ExecContext(ctx, upsertEarlyAdopterFeature,
		feature.UID, feature.OrganisationID, feature.FeatureKey,
		feature.Enabled, enabledBy, enabledAt)
	if err != nil {
		return err
	}

	return nil
}

// DeleteEarlyAdopterFeature deletes an early adopter feature
func DeleteEarlyAdopterFeature(ctx context.Context, db database.Database, orgID, featureKey string) error {
	_, err := db.GetDB().ExecContext(ctx, deleteEarlyAdopterFeature, orgID, featureKey)
	return err
}
