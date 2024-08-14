package noop

import "context"

// Noop License is for testing only

type Licenser struct{}

func (l Licenser) FeatureListJSON() []byte {
	return []byte{}
}

func NewLicenser() *Licenser {
	return &Licenser{}
}

func (l Licenser) CanCreateOrg(ctx context.Context) (bool, error) {
	return true, nil
}

func (l Licenser) CanCreateOrgMember(ctx context.Context) (bool, error) {
	return true, nil
}

func (l Licenser) CanUseForwardProxy() bool {
	return true
}

func (l Licenser) CanExportPrometheusMetrics() bool {
	return true
}

func (l Licenser) AdvancedEndpointMgmt() bool {
	return true
}

func (l Licenser) AdvancedSubscriptions() bool {
	return true
}

func (l Licenser) Transformations() bool {
	return true
}

func (l Licenser) AsynqMonitoring() bool {
	return true
}

func (l Licenser) AdvancedRetentionPolicy() bool {
	return true
}

func (l Licenser) AdvancedMsgBroker() bool {
	return true
}

func (l Licenser) WebhookAnalytics() bool {
	return true
}

func (l Licenser) HADeployment() bool {
	return true
}

func (l Licenser) MutualTLS() bool {
	return true
}

func (l Licenser) SynchronousWebhooks() bool {
	return true
}
