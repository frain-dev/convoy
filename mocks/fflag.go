package mocks

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
)

var (
	ErrFeatureFlagNotFound         = errors.New("feature flag not found")
	ErrOverrideNotFound            = errors.New("override not found")
	ErrEarlyAdopterFeatureNotFound = errors.New("early adopter feature not found")
)

// MockFeatureFlagFetcher is a mock implementation of fflag.FeatureFlagFetcher
type MockFeatureFlagFetcher struct {
	FetchFeatureFlagFunc         func(ctx context.Context, key string) (*fflag.FeatureFlagInfo, error)
	FetchFeatureFlagOverrideFunc func(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error)
}

// FetchFeatureFlag calls the mock function if set, otherwise returns an error
func (m *MockFeatureFlagFetcher) FetchFeatureFlag(ctx context.Context, key string) (*fflag.FeatureFlagInfo, error) {
	if m.FetchFeatureFlagFunc != nil {
		return m.FetchFeatureFlagFunc(ctx, key)
	}
	return nil, ErrFeatureFlagNotFound
}

// FetchFeatureFlagOverride calls the mock function if set, otherwise returns an error
func (m *MockFeatureFlagFetcher) FetchFeatureFlagOverride(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error) {
	if m.FetchFeatureFlagOverrideFunc != nil {
		return m.FetchFeatureFlagOverrideFunc(ctx, ownerType, ownerID, featureFlagID)
	}
	return nil, ErrOverrideNotFound
}

// NewMockFeatureFlagFetcher returns a simple mock fetcher that returns "not found" for everything
// This makes the code fall back to system config, which is fine for most tests
func NewMockFeatureFlagFetcher() *MockFeatureFlagFetcher {
	return &MockFeatureFlagFetcher{
		FetchFeatureFlagFunc: func(ctx context.Context, key string) (*fflag.FeatureFlagInfo, error) {
			return nil, ErrFeatureFlagNotFound
		},
		FetchFeatureFlagOverrideFunc: func(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error) {
			return nil, ErrOverrideNotFound
		},
	}
}

// MockEarlyAdopterFeatureFetcher is a mock implementation of fflag.EarlyAdopterFeatureFetcher
type MockEarlyAdopterFeatureFetcher struct {
	FetchEarlyAdopterFeatureFunc func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error)
}

// FetchEarlyAdopterFeature calls the mock function if set, otherwise returns an error
func (m *MockEarlyAdopterFeatureFetcher) FetchEarlyAdopterFeature(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
	if m.FetchEarlyAdopterFeatureFunc != nil {
		return m.FetchEarlyAdopterFeatureFunc(ctx, orgID, featureKey)
	}
	return nil, ErrEarlyAdopterFeatureNotFound
}

// NewMockEarlyAdopterFeatureFetcherWithMTLSEnabled returns a mock fetcher that returns mTLS as enabled
func NewMockEarlyAdopterFeatureFetcherWithMTLSEnabled() *MockEarlyAdopterFeatureFetcher {
	return &MockEarlyAdopterFeatureFetcher{
		FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
			if featureKey == "mtls" {
				return &fflag.EarlyAdopterFeatureInfo{
					Enabled: true,
				}, nil
			}
			return nil, ErrEarlyAdopterFeatureNotFound
		},
	}
}

// NewMockEarlyAdopterFeatureFetcherWithMTLSDisabled returns a mock fetcher that returns mTLS as disabled
func NewMockEarlyAdopterFeatureFetcherWithMTLSDisabled() *MockEarlyAdopterFeatureFetcher {
	return &MockEarlyAdopterFeatureFetcher{
		FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
			if featureKey == "mtls" {
				return &fflag.EarlyAdopterFeatureInfo{
					Enabled: false,
				}, nil
			}
			return nil, ErrEarlyAdopterFeatureNotFound
		},
	}
}
