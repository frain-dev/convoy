package fflag

import (
	"context"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
)

var features = map[string]datastore.FlagType{}

type Controller struct {
	fflagRepo datastore.FFlagRepository
}

func NewController(ctx context.Context, fflagRepo datastore.FFlagRepository) (*Controller, error) {
	err := fflagRepo.ClearFlagTable(ctx)
	if err != nil {
		return nil, err
	}

	flags := make([]datastore.Flag, 0, len(features))
	for s, flagType := range features {
		flags = append(flags, datastore.Flag{
			UID:        ulid.Make().String(),
			FeatureKey: s,
			Type:       flagType,
		})
	}

	err = fflagRepo.SaveFlags(ctx, flags)
	return &Controller{fflagRepo: fflagRepo}, err
}

func (c *Controller) CanAccessFeature(key string, cfg *config.Configuration) bool {
	// check for this feature in our feature map
	flagType, ok := features[key]
	if !ok {
		return false
	}

	if flagType == datastore.ExperimentalFlagType {
		return cfg.FeatureFlag.Experimental
	}

	if flagType == datastore.AlphaFlagType {
		return cfg.FeatureFlag.Alpha
	}

	if flagType == datastore.BetaFlagType {
		return cfg.FeatureFlag.Beta
	}

	if flagType == datastore.GAFlagType {
		return cfg.FeatureFlag.GA
	}

	return false
}
