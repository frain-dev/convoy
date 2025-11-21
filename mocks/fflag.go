package mocks

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
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
	return nil, errors.New("feature flag not found")
}

// FetchFeatureFlagOverride calls the mock function if set, otherwise returns an error
func (m *MockFeatureFlagFetcher) FetchFeatureFlagOverride(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error) {
	if m.FetchFeatureFlagOverrideFunc != nil {
		return m.FetchFeatureFlagOverrideFunc(ctx, ownerType, ownerID, featureFlagID)
	}
	return nil, errors.New("override not found")
}

// NewMockFeatureFlagFetcherWithMTLSEnabled returns a mock fetcher that returns mTLS as enabled
func NewMockFeatureFlagFetcherWithMTLSEnabled() *MockFeatureFlagFetcher {
	return &MockFeatureFlagFetcher{
		FetchFeatureFlagFunc: func(ctx context.Context, key string) (*fflag.FeatureFlagInfo, error) {
			if key == "mtls" {
				return &fflag.FeatureFlagInfo{
					UID:           "test-uid",
					Enabled:       true,
					AllowOverride: true,
				}, nil
			}
			return nil, errors.New("feature flag not found")
		},
		FetchFeatureFlagOverrideFunc: func(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error) {
			return nil, errors.New("override not found")
		},
	}
}

// NewMockFeatureFlagFetcherWithMTLSDisabled returns a mock fetcher that returns mTLS as disabled
func NewMockFeatureFlagFetcherWithMTLSDisabled() *MockFeatureFlagFetcher {
	return &MockFeatureFlagFetcher{
		FetchFeatureFlagFunc: func(ctx context.Context, key string) (*fflag.FeatureFlagInfo, error) {
			if key == "mtls" {
				return &fflag.FeatureFlagInfo{
					UID:           "test-uid",
					Enabled:       false,
					AllowOverride: true,
				}, nil
			}
			return nil, errors.New("feature flag not found")
		},
		FetchFeatureFlagOverrideFunc: func(ctx context.Context, ownerType, ownerID, featureFlagID string) (*fflag.FeatureFlagOverrideInfo, error) {
			return nil, errors.New("override not found")
		},
	}
}
