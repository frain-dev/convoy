package keygen

import (
	"context"
	"fmt"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/keygen-sh/keygen-go/v3"
)

type KeygenLicenser struct {
	licenseKey string
	license    *keygen.License
	orgRepo    datastore.OrganisationRepository
	planType   PlanType
}

type Config struct {
	LicenseKey string
	orgRepo    datastore.OrganisationRepository
}

type (
	Feature  string
	PlanType string
)

const (
	CreateOrg               Feature = "create_org"
	CreateOrgMember         Feature = "create_org_member"
	UseForwardProxy         Feature = "use_forward_proxy"
	ExportPrometheusMetrics Feature = "export_prometheus_metrics"
	AdvancedEndpointMgmt    Feature = "advanced_endpoint_mgmt"
	AdvancedRetentionPolicy Feature = "advanced_retention_policy"
	AdvancedMsgBroker       Feature = "advanced_msg_broker"
	AdvancedSubscriptions   Feature = "advanced_subscriptions"
	Transformations         Feature = "transformations"
	HADeployment            Feature = "ha_deployment"
	WebhookAnalytics        Feature = "webhook_analytics"
	MutualTLS               Feature = "mutual_tls"
)

const (
	BusinessPlan   PlanType = "business"
	EnterprisePlan PlanType = "enterprise"
)

func (p PlanType) IsValid() bool {
	switch p {
	case BusinessPlan, EnterprisePlan:
		return true
	}
	return false
}

func init() {
	keygen.Account = "1fddcec8-8dd3-4d8d-9b16-215cac0f9b52"
	keygen.Product = "1f086ec9-a943-46ea-9da4-e62c2180c2f4"
	keygen.PublicKey = "e8601e48b69383ba520245fd07971e983d06d22c4257cfd82304601479cee788"
}

func NewKeygenLicenser(c *Config) (*KeygenLicenser, error) {
	keygen.LicenseKey = c.LicenseKey

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	l, err := keygen.Validate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to validate error: %v", err)
	}

	if l.Metadata == nil {
		return nil, fmt.Errorf("license has no metadata")
	}

	p := l.Metadata["plan_type"]
	if p == nil {
		return nil, fmt.Errorf("nil license metadata")
	}

	pt, ok := p.(string)
	if !ok {
		return nil, fmt.Errorf("license plan type is not a string")
	}

	planType := PlanType(pt)
	if !planType.IsValid() {
		return nil, fmt.Errorf("license plan type is not valid: %s", planType)
	}

	return &KeygenLicenser{
		licenseKey: c.LicenseKey,
		license:    l,
		orgRepo:    c.orgRepo,
		planType:   planType,
	}, nil
}

func (k *KeygenLicenser) Activate() {
}

func (k *KeygenLicenser) CanCreateOrg() bool {
	return true
}

func (k *KeygenLicenser) CanCreateOrgMember() bool {
	return true
}

func (k *KeygenLicenser) CanUseForwardProxy() bool {
	return true
}

func (k *KeygenLicenser) CanExportPrometheusMetrics() bool {
	return true
}

func (k *KeygenLicenser) AdvancedEndpointMgmt() bool {
	return true
}

func (k *KeygenLicenser) AdvancedRetentionPolicy() bool {
	return true
}

func (k *KeygenLicenser) AdvancedMsgBroker() bool {
	return true
}

func (k *KeygenLicenser) AdvancedSubscriptions() bool {
	return true
}

func (k *KeygenLicenser) Transformations() bool {
	return true
}

func (k *KeygenLicenser) HADeployment() bool {
	return true
}

func (k *KeygenLicenser) WebhookAnalytics() bool {
	return true
}

func (k *KeygenLicenser) MutualTLS() bool {
	return true
}
