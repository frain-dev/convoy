package noopfflag

type NoopFeatureFlag struct{}

func NewNoopFeatureFlag() *NoopFeatureFlag {
	return &NoopFeatureFlag{}
}

func (n *NoopFeatureFlag) IsEnabled(flagKey string, evaluate map[string]string) (bool, error) {
	return true, nil
}
