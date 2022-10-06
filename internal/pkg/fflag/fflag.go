package fflag

import (
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag/flipt"
	noopfflag "github.com/frain-dev/convoy/internal/pkg/fflag/noop"
)

const (
	fliptProvider = "flipt"
)

type FeatureFlag interface {
	IsEnabled(flagKey string, evaluate map[string]string) (bool, error)
}

func NewFeatureFlagClient(c config.Configuration) (FeatureFlag, error) {
	if c.FeatureFlag.Type == config.FeatureFlagProvider(fliptProvider) {
		client, err := flipt.NewFliptClient(c.FeatureFlag.Flipt.Host)
		return client, err
	}

	return noopfflag.NewNoopFeatureFlag(), nil
}
