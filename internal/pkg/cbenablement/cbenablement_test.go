package cbenablement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/clock"
)

const cbUID = "cb-flag-uid"

func envFlag(on bool) *fflag.FFlag {
	if on {
		return fflag.NewFFlag([]string{string(fflag.CircuitBreaker)})
	}
	return fflag.NewFFlag(nil)
}

// fetcher builds a mock with the instance flag state, an optional override, and
// the "any enabled override" answer.
func fetcher(instanceEnabled bool, override *bool, anyOverride bool) *mocks.MockFeatureFlagFetcher {
	return &mocks.MockFeatureFlagFetcher{
		FetchFeatureFlagFunc: func(context.Context, string) (*fflag.FeatureFlagInfo, error) {
			return &fflag.FeatureFlagInfo{UID: cbUID, Enabled: instanceEnabled}, nil
		},
		FetchFeatureFlagOverrideFunc: func(context.Context, string, string, string) (*fflag.FeatureFlagOverrideInfo, error) {
			if override == nil {
				return nil, mocks.ErrOverrideNotFound
			}
			return &fflag.FeatureFlagOverrideInfo{Enabled: *override}, nil
		},
		AnyEnabledOverrideFunc: func(context.Context, string) (bool, error) {
			return anyOverride, nil
		},
	}
}

func boolPtr(b bool) *bool { return &b }

func newResolver(env bool, f *mocks.MockFeatureFlagFetcher) *Resolver {
	return NewResolver(envFlag(env), f, clock.NewSimulatedClock(time.Now()), nil)
}

func TestResolver_EnabledForOrg(t *testing.T) {
	tests := []struct {
		name     string
		env      bool
		instance bool
		override *bool
		want     bool
	}{
		{name: "env on is the base", env: true, instance: false, override: nil, want: true},
		{name: "env off, instance on", env: false, instance: true, override: nil, want: true},
		{name: "disabled override wins over env+instance (cloud)", env: true, instance: true, override: boolPtr(false), want: false},
		{name: "enabled override with env+instance off", env: false, instance: false, override: boolPtr(true), want: true},
		{name: "all off", env: false, instance: false, override: nil, want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newResolver(tc.env, fetcher(tc.instance, tc.override, false))
			require.Equal(t, tc.want, r.EnabledForOrg(context.Background(), "org-1"))
		})
	}
}

func TestResolver_EnabledForOrg_FetcherErrorFailsClosed(t *testing.T) {
	f := &mocks.MockFeatureFlagFetcher{
		FetchFeatureFlagFunc: func(context.Context, string) (*fflag.FeatureFlagInfo, error) {
			return nil, errors.New("db down")
		},
	}

	// env off + DB error => fail closed
	require.False(t, newResolver(false, f).EnabledForOrg(context.Background(), "org-1"))
	// env on + DB error => env base still honored
	require.True(t, newResolver(true, f).EnabledForOrg(context.Background(), "org-1"))
}

func TestResolver_EnabledAnywhere(t *testing.T) {
	t.Run("env on short-circuits", func(t *testing.T) {
		r := newResolver(true, fetcher(false, nil, false))
		require.True(t, r.EnabledAnywhere(context.Background()))
	})

	t.Run("instance flag on", func(t *testing.T) {
		r := newResolver(false, fetcher(true, nil, false))
		require.True(t, r.EnabledAnywhere(context.Background()))
	})

	t.Run("any enabled override turns sampler on", func(t *testing.T) {
		r := newResolver(false, fetcher(false, nil, true))
		require.True(t, r.EnabledAnywhere(context.Background()))
	})

	t.Run("all off", func(t *testing.T) {
		r := newResolver(false, fetcher(false, nil, false))
		require.False(t, r.EnabledAnywhere(context.Background()))
	})
}

func TestResolver_EnabledAnywhere_OverrideCheckErrorFailsClosed(t *testing.T) {
	f := &mocks.MockFeatureFlagFetcher{
		FetchFeatureFlagFunc: func(context.Context, string) (*fflag.FeatureFlagInfo, error) {
			return &fflag.FeatureFlagInfo{UID: cbUID, Enabled: false}, nil
		},
		AnyEnabledOverrideFunc: func(context.Context, string) (bool, error) {
			return false, errors.New("db down")
		},
	}
	require.False(t, newResolver(false, f).EnabledAnywhere(context.Background()))
}

func TestResolver_TTLCaching(t *testing.T) {
	instanceEnabled := false
	f := &mocks.MockFeatureFlagFetcher{
		FetchFeatureFlagFunc: func(context.Context, string) (*fflag.FeatureFlagInfo, error) {
			return &fflag.FeatureFlagInfo{UID: cbUID, Enabled: instanceEnabled}, nil
		},
		FetchFeatureFlagOverrideFunc: func(context.Context, string, string, string) (*fflag.FeatureFlagOverrideInfo, error) {
			return nil, mocks.ErrOverrideNotFound
		},
	}

	simClock := clock.NewSimulatedClock(time.Now())
	r := NewResolver(envFlag(false), f, simClock, nil)

	require.False(t, r.EnabledForOrg(context.Background(), "org-1"))

	// Flip the underlying value; within the TTL the cached false is returned.
	instanceEnabled = true
	require.False(t, r.EnabledForOrg(context.Background(), "org-1"))

	// Advance past the TTL; the resolver re-reads and sees the new value.
	simClock.AdvanceTime(defaultTTL + time.Second)
	require.True(t, r.EnabledForOrg(context.Background(), "org-1"))
}
