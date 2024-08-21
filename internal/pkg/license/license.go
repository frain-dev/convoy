package license

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy/internal/pkg/license/keygen"
)

// Licenser interface provides methods to determine whether the specified license can utilise certain features in convoy.
type Licenser interface {
	CreateOrg(ctx context.Context) (bool, error)
	CreateUser(ctx context.Context) (bool, error)
	CreateProject(ctx context.Context) (bool, error)
	UseForwardProxy() bool
	CanExportPrometheusMetrics() bool
	AdvancedEndpointMgmt() bool
	AdvancedSubscriptions() bool
	Transformations() bool
	AsynqMonitoring() bool
	PortalLinks() bool

	// need more fleshing out
	AdvancedRetentionPolicy() bool
	AdvancedMsgBroker() bool
	WebhookAnalytics() bool
	HADeployment() bool
	MutualTLS() bool
	SynchronousWebhooks() bool

	FeatureListJSON() json.RawMessage
}

var _ Licenser = &keygen.Licenser{}

type Config struct {
	KeyGen keygen.Config
}

func NewLicenser(c *Config) (Licenser, error) {
	return keygen.NewKeygenLicenser(&c.KeyGen)
}
