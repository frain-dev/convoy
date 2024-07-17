package keygen

var businessFeatures = map[Feature]struct{}{
	CreateOrg:               {},
	CreateOrgMember:         {},
	UseForwardProxy:         {},
	ExportPrometheusMetrics: {},
	AdvancedEndpointMgmt:    {},
	AdvancedRetentionPolicy: {},
	AdvancedMsgBroker:       {},
	AdvancedSubscriptions:   {},
	Transformations:         {},
	HADeployment:            {},
	WebhookAnalytics:        {},
	MutualTLS:               {},
}
