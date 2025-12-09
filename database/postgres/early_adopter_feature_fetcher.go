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
		if errors.Is(err, ErrEarlyAdopterFeatureNotFound) {
			return nil, err
		}
		return nil, err
	}

	return &fflag.EarlyAdopterFeatureInfo{
		Enabled: feature.Enabled,
	}, nil
}
