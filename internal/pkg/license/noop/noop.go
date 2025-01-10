//go:build integration

package noop

import (
	"context"
	"encoding/json"
)

// Noop License is for testing only

type Licenser struct{}

func (Licenser) FeatureListJSON(_ context.Context) (json.RawMessage, error) {
	return []byte{}, nil
}

func NewLicenser() *Licenser {
	return &Licenser{}
}

func (Licenser) CreateOrg(_ context.Context) (bool, error) {
	return true, nil
}

func (Licenser) CreateUser(_ context.Context) (bool, error) {
	return true, nil
}

func (Licenser) CreateProject(_ context.Context) (bool, error) {
	return true, nil
}

func (Licenser) UseForwardProxy() bool {
	return true
}

func (Licenser) CanExportPrometheusMetrics() bool {
	return true
}

func (Licenser) AdvancedEndpointMgmt() bool {
	return true
}

func (Licenser) AdvancedSubscriptions() bool {
	return true
}

func (Licenser) Transformations() bool {
	return true
}

func (Licenser) AsynqMonitoring() bool {
	return true
}

func (Licenser) RetentionPolicy() bool {
	return true
}

func (Licenser) AdvancedMsgBroker() bool {
	return true
}

func (Licenser) WebhookAnalytics() bool {
	return true
}

func (Licenser) HADeployment() bool {
	return true
}

func (Licenser) MutualTLS() bool {
	return true
}

func (Licenser) SynchronousWebhooks() bool {
	return true
}

func (Licenser) RemoveEnabledProject(_ string) {}

func (Licenser) ProjectEnabled(_ string) bool {
	return true
}

func (Licenser) AddEnabledProject(_ string) {}

func (Licenser) ConsumerPoolTuning() bool {
	return true
}

func (Licenser) AdvancedWebhookFiltering() bool {
	return true
}

func (Licenser) PortalLinks() bool {
	return true
}

func (Licenser) CircuitBreaking() bool {
	return true
}

func (Licenser) MultiPlayerMode() bool {
	return true
}

func (Licenser) IngestRate() bool {
	return true
}

func (Licenser) AgentExecutionMode() bool {
	return true
}

func (Licenser) IpRules() bool {
	return true
}

func (Licenser) EnterpriseSSO() bool {
	return true
}

func (k *Licenser) DatadogTracing() bool {
	return true
}
func (k *Licenser) ReadReplica() bool {
	return true
}
func (k *Licenser) CredentialEncryption() bool {
	return true
}
