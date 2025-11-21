package postgres

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/database"
	fflag "github.com/frain-dev/convoy/internal/pkg/fflag"
)

// FeatureFlagFetcherImpl implements fflag.FeatureFlagFetcher
type FeatureFlagFetcherImpl struct {
	db database.Database
}

// NewFeatureFlagFetcher creates a new FeatureFlagFetcher
func NewFeatureFlagFetcher(db database.Database) fflag.FeatureFlagFetcher {
	return &FeatureFlagFetcherImpl{db: db}
}

// FetchFeatureFlag fetches a feature flag by key
func (f *FeatureFlagFetcherImpl) FetchFeatureFlag(ctx context.Context, key string) (*fflag.FeatureFlagInfo, error) {
	flag, err := FetchFeatureFlagByKey(ctx, f.db, key)
	if err != nil {
		if errors.Is(err, ErrFeatureFlagNotFound) {
			return nil, err
		}
		return nil, err
	}

	return &fflag.FeatureFlagInfo{
		UID:           flag.UID,
		Enabled:       flag.Enabled,
		AllowOverride: flag.AllowOverride,
	}, nil
}

// FetchFeatureFlagOverride fetches a feature flag override
func (f *FeatureFlagFetcherImpl) FetchFeatureFlagOverride(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error) {
	override, err := FetchFeatureFlagOverride(ctx, f.db, ownerType, ownerID, featureFlagID)
	if err != nil {
		return nil, err
	}

	return &fflag.FeatureFlagOverrideInfo{
		Enabled: override.Enabled,
	}, nil
}
