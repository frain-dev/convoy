package fflag

import (
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

type (
	FeatureFlagKey string
)

const (
	IpRules              FeatureFlagKey = "ip-rules"
	Prometheus           FeatureFlagKey = "prometheus"
	CircuitBreaker       FeatureFlagKey = "circuit-breaker"
	FullTextSearch       FeatureFlagKey = "full-text-search"
	RetentionPolicy      FeatureFlagKey = "retention-policy"
	CredentialEncryption FeatureFlagKey = "credential-encryption"
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
	CredentialEncryption: disabled,
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
		case string(CredentialEncryption):
			f.Features[CredentialEncryption] = enabled
		}
	}

	return f
}

func clone(src map[FeatureFlagKey]FeatureFlagState) map[FeatureFlagKey]FeatureFlagState {
	dst := make(map[FeatureFlagKey]FeatureFlagState)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (c *FFlag) CanAccessFeature(key FeatureFlagKey) bool {
	// check for this feature in our feature map
	state, ok := c.Features[key]
	if !ok {
		return false
	}

	return bool(state)
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
