package loader

// Configuration constants
const (
	DefaultBatchSize = 10_000
)

// LoaderConfig holds configuration for the subscription loader
type LoaderConfig struct {
	BatchSize   int64
	EnableDebug bool
}

// NewLoaderConfig creates a new loader configuration with defaults
func NewLoaderConfig(batchSize int64, enableDebug bool) *LoaderConfig {
	if batchSize == 0 {
		batchSize = DefaultBatchSize
	}

	return &LoaderConfig{
		BatchSize:   batchSize,
		EnableDebug: enableDebug,
	}
}

// Validate ensures the configuration is valid
func (c *LoaderConfig) Validate() error {
	if c.BatchSize <= 0 {
		return ErrInvalidBatchSize
	}
	return nil
}
