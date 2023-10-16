package fflag

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
)

var features = map[string]datastore.FlagType{}

type FFlag struct{}

func NewFFlag() *FFlag {
	return &FFlag{}
}

func (c *FFlag) CanAccessFeature(key string, cfg *config.Configuration) bool {
	// check for this feature in our feature map
	flagType, ok := features[key]
	if !ok {
		return false
	}

	switch flagType {
	case datastore.ExperimentalFlagType:
		return cfg.FeatureFlag.Experimental
	default:
		return false
	}
}
