package keygen

type (
	Feature  string
	PlanType string
)

const (
	CreateOrg               Feature = "CREATE_ORG"
	CreateOrgMember         Feature = "CREATE_ORG_MEMBER"
	UseForwardProxy         Feature = "USE_FORWARD_PROXY"
	ExportPrometheusMetrics Feature = "EXPORT_PROMETHEUS_METRICS"
	AdvancedEndpointMgmt    Feature = "ADVANCED_ENDPOINT_MANAGEMENT"
	AdvancedRetentionPolicy Feature = "ADVANCED_RETENTION_POLICY"
	AdvancedMsgBroker       Feature = "ADVANCED_MESSAGE_BROKER"
	AdvancedSubscriptions   Feature = "ADVANCED_SUBSCRIPTIONS"
	Transformations         Feature = "TRANSFORMATIONS"
	HADeployment            Feature = "HA_DEPLOYMENT"
	WebhookAnalytics        Feature = "WEBHOOK_ANALYTICS"
	MutualTLS               Feature = "MUTUAL_TLS"
	AsynqMonitoring         Feature = "ASYNQ_MONITORING"
	SynchronousWebhooks     Feature = "SYNCHRONOUS_WEBHOOKS"
)

const (
	CommunityPlan  PlanType = "community"
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
