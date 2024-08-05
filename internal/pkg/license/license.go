package license

import (
	"context"

	"github.com/frain-dev/convoy/internal/pkg/license/keygen"
)

// Licenser interface provides methods to determine whether the specified license can utilise certain features in convoy.
type Licenser interface {
	CanCreateOrg(ctx context.Context) (bool, error)
	CanCreateOrgMember(ctx context.Context) (bool, error)
	CanUseForwardProxy() bool
	CanExportPrometheusMetrics() bool
	AdvancedEndpointMgmt() bool
	AdvancedSubscriptions() bool
	Transformations() bool
	AsynqMonitoring() bool

	// need more fleshing out
	AdvancedRetentionPolicy() bool
	AdvancedMsgBroker() bool
	WebhookAnalytics() bool
	HADeployment() bool
	MutualTLS() bool
	SynchronousWebhooks() bool
}

var _ Licenser = &keygen.KeygenLicenser{}

type Config struct {
	KeyGen keygen.Config
}

func NewLicenser(c *Config) (Licenser, error) {
	return keygen.NewKeygenLicenser(&c.KeyGen)
}
