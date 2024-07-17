package license

import "github.com/frain-dev/convoy/internal/pkg/license/keygen"

// Licenser interface provides methods to determine whether the specified license can utilise certain features in convoy.
type Licenser interface {
	CanCreateOrg() bool
	CanCreateOrgMember() bool
	CanUseForwardProxy() bool
	CanExportPrometheusMetrics() bool
	AdvancedEndpointMgmt() bool
	AdvancedRetentionPolicy() bool
	AdvancedMsgBroker() bool
	AdvancedSubscriptions() bool
	Transformations() bool
	HADeployment() bool // needs more fleshing out
	WebhookAnalytics() bool
	MutualTLS() bool // needs more fleshing out
	// SynchronousWebhooks() bool
}

type Config struct {
	KeyGen keygen.Config
}

func NewLicenser(c *Config) (Licenser, error) {
	return keygen.NewKeygenLicenser(c.KeyGen), nil
}
