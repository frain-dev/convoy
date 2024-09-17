package fflag

import (
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"os"
	"sort"
	"text/tabwriter"
)

var ErrCircuitBreakerNotEnabled = errors.New("[feature flag] circuit breaker is not enabled")
var ErrFullTextSearchNotEnabled = errors.New("[feature flag] full text search is not enabled")
var ErrPrometheusMetricsNotEnabled = errors.New("[feature flag] prometheus metrics is not enabled")

type (
	FeatureFlagKey string
)

const (
	Prometheus     FeatureFlagKey = "prometheus"
	FullTextSearch FeatureFlagKey = "full-text-search"
	CircuitBreaker FeatureFlagKey = "circuit-breaker"
)

type (
	FeatureFlagState bool
)

const (
	enabled  FeatureFlagState = true
	disabled FeatureFlagState = false
)

var DefaultFeaturesState = map[FeatureFlagKey]FeatureFlagState{
	Prometheus:     disabled,
	FullTextSearch: disabled,
	CircuitBreaker: disabled,
}

type FFlag struct {
	Features map[FeatureFlagKey]FeatureFlagState
}

func NewFFlag(c *config.Configuration) *FFlag {
	f := &FFlag{
		Features: clone(DefaultFeaturesState),
	}

	for _, flag := range c.EnableFeatureFlag {
		switch flag {
		case string(Prometheus):
			f.Features[Prometheus] = enabled
		case string(FullTextSearch):
			f.Features[FullTextSearch] = enabled
		case string(CircuitBreaker):
			f.Features[CircuitBreaker] = enabled
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

		_, err := fmt.Fprintf(w, "%s\t%s\n", k, state)
		if err != nil {
			return err
		}
	}

	return w.Flush()
}
