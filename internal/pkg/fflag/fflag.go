package fflag

import (
	"github.com/frain-dev/convoy/config"
)

type FlagType string

const (
	ExperimentalFlagType FlagType = "experimental"
)

var flagLevels = map[string]int{
	ExperimentalFlagType.String(): 1,
}

func (ft FlagType) String() string {
	return string(ft)
}

var features = map[string]FlagType{}

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

	lvl := flagLevels[flagType.String()]
	if lvl == 0 {
		return false
	}

	cfgLvl := flagLevels[cfg.FeatureFlag]

	return lvl <= cfgLvl // if the feature level is less than or equal to the cfg level, we can access the feature
}
