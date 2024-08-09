package keygen

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
	AsynqMonitoring         Feature = "asynq_monitoring"
	SynchronousWebhooks     Feature = "synchronous_webhooks"
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

// Properties will hold characteristics for features like organisation
// number limit, but it can also be empty, because certain feature don't need them
type Properties struct {
	Limit int64 `mapstructure:"limit"`
}
