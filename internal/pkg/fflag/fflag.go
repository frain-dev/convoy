package fflag

import (
	"github.com/frain-dev/convoy/config"
)

type (
	FeatureFlagKey string
)

const (
	Prometheus FeatureFlagKey = "prometheus"
)

var features = map[FeatureFlagKey]config.FlagLevel{
	Prometheus: config.ExperimentalFlagLevel,
}

type FFlag struct{}

func NewFFlag() (*FFlag, error) {
	return &FFlag{}, nil
}

func (c *FFlag) CanAccessFeature(key FeatureFlagKey, cfg *config.Configuration) bool {
	// check for this feature in our feature map
	flagLevel, ok := features[key]
	if !ok {
		return false
	}

	return flagLevel <= cfg.FeatureFlag // if the feature level is less than or equal to the cfg level, we can access the feature
}
