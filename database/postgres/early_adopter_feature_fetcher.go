package postgres

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/database"
	fflag "github.com/frain-dev/convoy/internal/pkg/fflag"
)

// EarlyAdopterFeatureFetcherImpl implements fflag.EarlyAdopterFeatureFetcher
type EarlyAdopterFeatureFetcherImpl struct {
	db database.Database
}

// NewEarlyAdopterFeatureFetcher creates a new EarlyAdopterFeatureFetcher
func NewEarlyAdopterFeatureFetcher(db database.Database) fflag.EarlyAdopterFeatureFetcher {
	return &EarlyAdopterFeatureFetcherImpl{db: db}
}

// FetchEarlyAdopterFeature fetches an early adopter feature for an organisation
func (f *EarlyAdopterFeatureFetcherImpl) FetchEarlyAdopterFeature(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
	feature, err := FetchEarlyAdopterFeature(ctx, f.db, orgID, featureKey)
	if err != nil {
		// Map "no row" onto the fflag sentinel so callers can classify it as a
		// definitive "off" without importing this package.
		if errors.Is(err, ErrEarlyAdopterFeatureNotFound) {
			return nil, fflag.ErrEarlyAdopterFeatureNotFound
		}
		return nil, err
	}

	return &fflag.EarlyAdopterFeatureInfo{
		Enabled: feature.Enabled,
	}, nil
}
