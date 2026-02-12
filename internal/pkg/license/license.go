package license

import (
	"context"
	"encoding/json"

	"github.com/frain-dev/convoy/internal/pkg/license/service"
)

// Licenser interface provides methods to determine whether the specified license can utilise certain features in convoy.
type Licenser interface {
	// Limit check methods
	CheckOrgLimit(ctx context.Context) (bool, error)
	CheckUserLimit(ctx context.Context) (bool, error)
	CheckProjectLimit(ctx context.Context) (bool, error)
	IsMultiUserMode(ctx context.Context) (bool, error)

	UseForwardProxy() bool
	CanExportPrometheusMetrics() bool
	AdvancedEndpointMgmt() bool
	AdvancedSubscriptions() bool
	Transformations() bool
	AsynqMonitoring() bool
	PortalLinks() bool
	ConsumerPoolTuning() bool
	AdvancedWebhookFiltering() bool
	CircuitBreaking() bool
	IngestRate() bool
	AgentExecutionMode() bool
	IpRules() bool
	EnterpriseSSO() bool
	GoogleOAuth() bool
	DatadogTracing() bool
	ReadReplica() bool
	CredentialEncryption() bool
	CustomCertificateAuthority() bool
	StaticIP() bool

	RetentionPolicy() bool
	WebhookAnalytics() bool
	MutualTLS() bool
	OAuth2EndpointAuth() bool
	FeatureListJSON(ctx context.Context) (json.RawMessage, error)

	RemoveEnabledProject(projectID string)
	AddEnabledProject(projectID string)
	ProjectEnabled(projectID string) bool
}

var _ Licenser = &service.Licenser{}

type Config struct {
	LicenseService service.LicenserConfig
}

func NewLicenser(c *Config) (Licenser, error) {
	return service.NewLicenser(c.LicenseService)
}
