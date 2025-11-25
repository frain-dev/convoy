package fflag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
)

var ErrCircuitBreakerNotEnabled = errors.New("[feature flag] circuit breaker is not enabled")
var ErrFullTextSearchNotEnabled = errors.New("[feature flag] full text search is not enabled")
var ErrRetentionPolicyNotEnabled = errors.New("[feature flag] retention policy is not enabled")
var ErrPrometheusMetricsNotEnabled = errors.New("[feature flag] prometheus metrics is not enabled")
var ErrCredentialEncryptionNotEnabled = errors.New("[feature flag] credential encryption is not enabled")
var ErrMTLSNotEnabled = errors.New("[feature flag] mTLS is not enabled")
var ErrOAuthTokenExchangeNotEnabled = errors.New("[feature flag] OAuth token exchange is not enabled")

type (
	FeatureFlagKey string
)

const (
	IpRules              FeatureFlagKey = "ip-rules"
	Prometheus           FeatureFlagKey = "prometheus"
	CircuitBreaker       FeatureFlagKey = "circuit-breaker"
	FullTextSearch       FeatureFlagKey = "full-text-search"
	RetentionPolicy      FeatureFlagKey = "retention-policy"
	ReadReplicas         FeatureFlagKey = "read-replicas"
	CredentialEncryption FeatureFlagKey = "credential-encryption"
	MTLS                 FeatureFlagKey = "mtls"
	OAuthTokenExchange   FeatureFlagKey = "oauth-token-exchange"
)

type (
	FeatureFlagState bool
)

const (
	enabled  FeatureFlagState = true
	disabled FeatureFlagState = false
)

var DefaultFeaturesState = map[FeatureFlagKey]FeatureFlagState{
	IpRules:              disabled,
	Prometheus:           disabled,
	FullTextSearch:       disabled,
	CircuitBreaker:       disabled,
	RetentionPolicy:      disabled,
	ReadReplicas:         disabled,
	CredentialEncryption: disabled,
	MTLS:                 disabled,
	OAuthTokenExchange:   disabled,
}

type FFlag struct {
	Features map[FeatureFlagKey]FeatureFlagState
}

func NewFFlag(enableFeatureFlags []string) *FFlag {
	f := &FFlag{
		Features: clone(DefaultFeaturesState),
	}

	for _, flag := range enableFeatureFlags {
		switch flag {
		case string(IpRules):
			f.Features[IpRules] = enabled
		case string(Prometheus):
			f.Features[Prometheus] = enabled
		case string(FullTextSearch):
			f.Features[FullTextSearch] = enabled
		case string(CircuitBreaker):
			f.Features[CircuitBreaker] = enabled
		case string(RetentionPolicy):
			f.Features[RetentionPolicy] = enabled
		case string(ReadReplicas):
			f.Features[ReadReplicas] = enabled
		case string(CredentialEncryption):
			f.Features[CredentialEncryption] = enabled
		case string(MTLS):
			f.Features[MTLS] = enabled
		case string(OAuthTokenExchange):
			f.Features[OAuthTokenExchange] = enabled
		}
	}

	return f
}

func NoopFflag() *FFlag {
	return &FFlag{
		Features: clone(DefaultFeaturesState),
	}
}

func clone(src map[FeatureFlagKey]FeatureFlagState) map[FeatureFlagKey]FeatureFlagState {
	dst := make(map[FeatureFlagKey]FeatureFlagState)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// CanAccessFeature checks if a system-wide feature is enabled
// This only checks system-wide configuration (CLI/environment variables)
// For org-level feature flags, use CanAccessOrgFeature instead
func (c *FFlag) CanAccessFeature(key FeatureFlagKey) bool {
	state, ok := c.Features[key]
	if !ok {
		return false
	}

	return bool(state)
}

// CanAccessOrgFeature checks if an org-level feature flag is enabled for an organization
func (c *FFlag) CanAccessOrgFeature(ctx context.Context, key FeatureFlagKey, fetcher FeatureFlagFetcher, orgID string) bool {
	if key == Prometheus || key == ReadReplicas {
		return c.CanAccessFeature(key)
	}

	if fetcher == nil {
		return c.CanAccessFeature(key)
	}

	flagInfo, err := fetcher.FetchFeatureFlag(ctx, string(key))
	if err != nil {
		return c.CanAccessFeature(key)
	}

	var overrideInfo *FeatureFlagOverrideInfo
	if flagInfo.AllowOverride {
		overrideInfo, _ = fetcher.FetchFeatureFlagOverride(ctx, "organisation", orgID, flagInfo.UID)
	}

	data := &FeatureFlagData{
		FeatureFlag: flagInfo,
		Override:    overrideInfo,
	}

	return CanAccessFeatureWithOrg(c, key, data)
}

// FeatureFlagData contains the feature flag and optional override data
type FeatureFlagData struct {
	FeatureFlag *FeatureFlagInfo
	Override    *FeatureFlagOverrideInfo
}

// FeatureFlagInfo contains feature flag information from the database
type FeatureFlagInfo struct {
	UID           string
	Enabled       bool
	AllowOverride bool
}

// FeatureFlagOverrideInfo contains override information from the database
type FeatureFlagOverrideInfo struct {
	Enabled bool
}

// FeatureFlagFetcher is an interface for fetching feature flags from the database
type FeatureFlagFetcher interface {
	FetchFeatureFlag(ctx context.Context, key string) (*FeatureFlagInfo, error)
	FetchFeatureFlagOverride(ctx context.Context, ownerType, ownerID, featureFlagID string) (*FeatureFlagOverrideInfo, error)
}

// CanAccessFeatureWithOrg checks if a feature is enabled for an organization
func CanAccessFeatureWithOrg(systemFFlag *FFlag, key FeatureFlagKey, data *FeatureFlagData) bool {
	if key == Prometheus || key == ReadReplicas {
		state, ok := systemFFlag.Features[key]
		if !ok {
			return false
		}
		return bool(state)
	}

	if data == nil || data.FeatureFlag == nil {
		state, ok := systemFFlag.Features[key]
		if !ok {
			return false
		}
		return bool(state)
	}

	if !data.FeatureFlag.AllowOverride {
		return data.FeatureFlag.Enabled
	}

	if data.Override != nil {
		return data.Override.Enabled
	}

	return data.FeatureFlag.Enabled
}

// EarlyAdopterFeatures defines which features are available under Early Adopter program
var EarlyAdopterFeatures = []FeatureFlagKey{
	MTLS,
	OAuthTokenExchange,
}

// GetEarlyAdopterFeatures returns the list of features available under Early Adopter
func GetEarlyAdopterFeatures() []FeatureFlagKey {
	return EarlyAdopterFeatures
}

// IsEarlyAdopterFeature checks if a feature is part of the Early Adopter program
func IsEarlyAdopterFeature(key FeatureFlagKey) bool {
	for _, feature := range EarlyAdopterFeatures {
		if feature == key {
			return true
		}
	}
	return false
}

func (c *FFlag) ListFeatures() error {
	keys := make([]string, 0, len(c.Features))

	for k := range c.Features {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	_, err := fmt.Fprintln(w, "Features\tState")
	if err != nil {
		return err
	}

	for _, k := range keys {
		stateBool := c.Features[FeatureFlagKey(k)]
		state := "disabled"
		if stateBool {
			state = "enabled"
		}

		_, err = fmt.Fprintf(w, "%s\t%s\n", k, state)
		if err != nil {
			return err
		}
	}

	return w.Flush()
}
