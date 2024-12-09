package keygen

type (
	Feature  string
	PlanType string
)

const (
	CreateOrg                Feature = "CREATE_ORG"
	CreateUser               Feature = "CREATE_USER"
	CreateProject            Feature = "CREATE_PROJECT"
	UseForwardProxy          Feature = "USE_FORWARD_PROXY"
	ExportPrometheusMetrics  Feature = "EXPORT_PROMETHEUS_METRICS"
	AdvancedEndpointMgmt     Feature = "ADVANCED_ENDPOINT_MANAGEMENT"
	AdvancedWebhookArchiving Feature = "ADVANCED_WEBHOOK_ARCHIVING"
	AdvancedMsgBroker        Feature = "ADVANCED_MESSAGE_BROKER"
	AdvancedSubscriptions    Feature = "ADVANCED_SUBSCRIPTIONS"
	WebhookTransformations   Feature = "WEBHOOK_TRANSFORMATIONS"
	HADeployment             Feature = "HA_DEPLOYMENT"
	WebhookAnalytics         Feature = "WEBHOOK_ANALYTICS"
	MutualTLS                Feature = "MUTUAL_TLS"
	AsynqMonitoring          Feature = "ASYNQ_MONITORING"
	SynchronousWebhooks      Feature = "SYNCHRONOUS_WEBHOOKS"
	PortalLinks              Feature = "PORTAL_LINKS"
	ConsumerPoolTuning       Feature = "CONSUMER_POOL_TUNING"
	AdvancedWebhookFiltering Feature = "ADVANCED_WEBHOOK_FILTERING"
	CircuitBreaking          Feature = "CIRCUIT_BREAKING"
	MultiPlayerMode          Feature = "MULTI_PLAYER_MODE"
	IngestRate               Feature = "INGEST_RATE"
	AgentExecutionMode       Feature = "AGENT_EXECUTION_MODE"
	IpRules                  Feature = "IP_RULES"
	EnterpriseSSO            Feature = "ENTERPRISE_SSO"
	DatadogTracing           Feature = "DATADOG_TRACING"
	ReadReplica              Feature = "READ_REPLICA"
	CredentialEncryption     Feature = "CREDENTIAL_ENCRYPTION"
)

const (
	CommunityPlan  PlanType = "community"
	BusinessPlan   PlanType = "business"
	EnterprisePlan PlanType = "enterprise"
)

type Properties struct {
	Limit   int64 `mapstructure:"limit" json:"-"`
	Allowed bool  `json:"allowed"`
}

type LicenseMetadata struct {
	UserLimit    int64 `mapstructure:"userLimit"`
	OrgLimit     int64 `mapstructure:"orgLimit"`
	ProjectLimit int64 `mapstructure:"projectLimit"`
}
