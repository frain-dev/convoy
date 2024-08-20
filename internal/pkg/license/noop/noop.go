//go:build integration

package noop

import (
	"context"
	"encoding/json"
)

// Noop License is for testing only

type Licenser struct{}

func (Licenser) FeatureListJSON() json.RawMessage {
	return []byte{}
}

func NewLicenser() *Licenser {
	return &Licenser{}
}

func (Licenser) CreateOrg(ctx context.Context) (bool, error) {
	return true, nil
}

func (Licenser) CreateOrgMember(ctx context.Context) (bool, error) {
	return true, nil
}

func (Licenser) CreateProject(ctx context.Context) (bool, error) {
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

func (Licenser) AdvancedRetentionPolicy() bool {
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
