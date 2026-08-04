package fflag

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/frain-dev/convoy/config"
)

func TestFFlag_CanAccessFeature(t *testing.T) {
	type fields struct {
		Features map[FeatureFlagKey]FeatureFlagState
	}
	type args struct {
		key FeatureFlagKey
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "default state - no prometheus",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           disabled,
					IpRules:              disabled,
					FullTextSearch:       enabled,
					CircuitBreaker:       disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: Prometheus,
			},
			want: false,
		},
		{
			name: "default state - search available",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           disabled,
					IpRules:              disabled,
					FullTextSearch:       enabled,
					CircuitBreaker:       disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: FullTextSearch,
			},
			want: true,
		},
		{
			name: "all enabled state - prometheus available",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           enabled,
					IpRules:              disabled,
					FullTextSearch:       enabled,
					CircuitBreaker:       disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: Prometheus,
			},
			want: true,
		},
		{
			name: "all enabled state - search available",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           enabled,
					IpRules:              disabled,
					FullTextSearch:       enabled,
					CircuitBreaker:       disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: FullTextSearch,
			},
			want: true,
		},
		{
			name: "all disabled state - no prometheus",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           disabled,
					FullTextSearch:       disabled,
					CircuitBreaker:       disabled,
					IpRules:              disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: Prometheus,
			},
			want: false,
		},
		{
			name: "all disabled state - no search",
			fields: struct {
				Features map[FeatureFlagKey]FeatureFlagState
			}{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           disabled,
					FullTextSearch:       disabled,
					CircuitBreaker:       disabled,
					IpRules:              disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
				},
			},
			args: struct {
				key FeatureFlagKey
			}{
				key: FullTextSearch,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &FFlag{
				Features: tt.fields.Features,
			}
			if got := c.CanAccessFeature(tt.args.key); got != tt.want {
				t.Errorf("CanAccessFeature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewFFlag(t *testing.T) {
	type args struct {
		c *config.Configuration
	}
	tests := []struct {
		name    string
		args    args
		want    *FFlag
		wantErr bool
	}{
		{
			name: "default state",
			args: args{
				&config.Configuration{},
			},
			want: &FFlag{
				Features: DefaultFeaturesState,
			},
			wantErr: false,
		},
		{
			name: "default state - assert all disabled",
			args: args{
				&config.Configuration{},
			},
			want: &FFlag{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           disabled,
					FullTextSearch:       disabled,
					CircuitBreaker:       disabled,
					IpRules:              disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
					MTLS:                 disabled,
					OAuthTokenExchange:   disabled,
					BasicAuthEndpoint:    disabled,
					EndpointURLTemplates: disabled,
				},
			},
			wantErr: false,
		},
		{
			name: "enabled state - prometheus only",
			args: args{
				&config.Configuration{
					EnableFeatureFlag: []string{"prometheus"},
				},
			},
			want: &FFlag{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           enabled,
					FullTextSearch:       disabled,
					CircuitBreaker:       disabled,
					IpRules:              disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
					MTLS:                 disabled,
					OAuthTokenExchange:   disabled,
					BasicAuthEndpoint:    disabled,
					EndpointURLTemplates: disabled,
				},
			},
			wantErr: false,
		},
		{
			name: "all disabled state - by default",
			args: args{
				&config.Configuration{},
			},
			want: &FFlag{
				Features: map[FeatureFlagKey]FeatureFlagState{
					Prometheus:           disabled,
					FullTextSearch:       disabled,
					CircuitBreaker:       disabled,
					IpRules:              disabled,
					ReadReplicas:         disabled,
					CredentialEncryption: disabled,
					MTLS:                 disabled,
					OAuthTokenExchange:   disabled,
					BasicAuthEndpoint:    disabled,
					EndpointURLTemplates: disabled,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFFlag(tt.args.c.EnableFeatureFlag)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewFFlag() got = %v, want %v", got, tt.want)
			}
		})
	}
}

type stubEarlyAdopterFetcher struct {
	info *EarlyAdopterFeatureInfo
	err  error
}

func (s stubEarlyAdopterFetcher) FetchEarlyAdopterFeature(context.Context, string, string) (*EarlyAdopterFeatureInfo, error) {
	return s.info, s.err
}

// The resolver is the single place that decides what a missing row means, so
// both the endpoint create path and the dynamic worker inherit one verdict.
func TestResolveEarlyAdopterFeature(t *testing.T) {
	lookupErr := errors.New("feature store unavailable")

	tests := []struct {
		name        string
		fetcher     stubEarlyAdopterFetcher
		wantEnabled bool
		wantErr     error
	}{
		{
			name:        "enabled row",
			fetcher:     stubEarlyAdopterFetcher{info: &EarlyAdopterFeatureInfo{Enabled: true}},
			wantEnabled: true,
		},
		{
			name:    "disabled row",
			fetcher: stubEarlyAdopterFetcher{info: &EarlyAdopterFeatureInfo{Enabled: false}},
		},
		{
			name:    "missing row is a definitive off, not a lookup failure",
			fetcher: stubEarlyAdopterFetcher{err: ErrEarlyAdopterFeatureNotFound},
		},
		{
			name:    "nil info without an error is treated as off",
			fetcher: stubEarlyAdopterFetcher{},
		},
		{
			name:    "lookup failure propagates so callers can retry",
			fetcher: stubEarlyAdopterFetcher{err: lookupErr},
			wantErr: lookupErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enabled, err := ResolveEarlyAdopterFeature(context.Background(), EndpointURLTemplates, tt.fetcher, "org-id-1")

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ResolveEarlyAdopterFeature() err = %v, want %v", err, tt.wantErr)
			}
			if enabled != tt.wantEnabled {
				t.Errorf("ResolveEarlyAdopterFeature() enabled = %v, want %v", enabled, tt.wantEnabled)
			}
		})
	}
}

// A lookup failure must not grant access on the create/update path even though
// the same failure is retryable for the worker.
func TestCanAccessOrgFeatureFailsClosedOnLookupError(t *testing.T) {
	f := NoopFflag()
	f.Features[EndpointURLTemplates] = enabled

	fetcher := stubEarlyAdopterFetcher{err: errors.New("feature store unavailable")}
	if f.CanAccessOrgFeature(context.Background(), EndpointURLTemplates, nil, fetcher, "org-id-1") {
		t.Error("CanAccessOrgFeature() = true, want false when the lookup fails")
	}
}
