package fflag

import (
	"github.com/frain-dev/convoy/config"
)

var features = map[string]config.FlagLevel{}

type FFlag struct{}

func NewFFlag() *FFlag {
	return &FFlag{}
}

func (c *FFlag) CanAccessFeature(key string, cfg *config.Configuration) bool {
	// check for this feature in our feature map
	flagLevel, ok := features[key]
	if !ok {
		return false
	}

	return flagLevel <= cfg.FeatureFlag // if the feature level is less than or equal to the cfg level, we can access the feature
}
