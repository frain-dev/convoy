package license

import (
	"encoding/json"

	"github.com/frain-dev/convoy/internal/pkg/license/service"
)

// FeatureListFromEntitlements builds the same JSON shape as Licenser.FeatureListJSON
// from a raw entitlements map (e.g. from org license_data). Used when returning
// per-org features from stored entitlements. Limit objects use allowed=true,
// available=true, limit_reached=false, current=0 when a limit value is present.
func FeatureListFromEntitlements(entitlements map[string]interface{}) (json.RawMessage, error) {
	if len(entitlements) == 0 {
		return json.Marshal(map[string]interface{}{})
	}
	parsed := service.ParseEntitlements(entitlements)
	out := make(map[string]interface{})

	// Limit objects: same keys and shape as licenser FeatureListJSON
	for _, key := range []string{"org_limit", "user_limit", "project_limit"} {
		limit, exists := service.GetNumberEntitlement(parsed, key)
		if exists {
			available := limit > 0 || limit == -1
			out[key] = map[string]interface{}{
				"limit":         limit,
				"allowed":       true,
				"current":       int64(0),
				"available":     available,
				"limit_reached": false,
			}
		}
	}

	// Boolean features: keys must match licenser FeatureListJSON output
	featureKeys := []struct {
		entitlementKey string
		outputKey      string
	}{
		{"enterprise_sso", "EnterpriseSSO"},
		{"portal_links", "PortalLinks"},
		{"webhook_transformations", "Transformations"},
		{"advanced_subscriptions", "AdvancedSubscriptions"},
		{"webhook_analytics", "WebhookAnalytics"},
		{"advanced_webhook_filtering", "AdvancedWebhookFiltering"},
		{"advanced_endpoint_mgmt", "AdvancedEndpointMgmt"},
		{"circuit_breaking", "CircuitBreaking"},
		{"consumer_pool_tuning", "ConsumerPoolTuning"},
		{"google_oauth", "GoogleOAuth"},
		{"export_prometheus_metrics", "CanExportPrometheusMetrics"},
		{"read_replica", "ReadReplica"},
		{"credential_encryption", "CredentialEncryption"},
		{"ip_rules", "IpRules"},
		{"webhook_archiving", "RetentionPolicy"},
		{"mutual_tls", "MutualTLS"},
		{"datadog_tracing", "DatadogTracing"},
		{"custom_certificate_authority", "CustomCertificateAuthority"},
		{"static_ip", "StaticIP"},
		{"oauth2_endpoint_auth", "OAuth2EndpointAuth"},
		{"use_forward_proxy", "UseForwardProxy"},
		{"asynq_monitoring", "AsynqMonitoring"},
		{"agent_execution_mode", "AgentExecutionMode"},
	}
	for _, f := range featureKeys {
		out[f.outputKey] = service.GetBoolEntitlement(parsed, f.entitlementKey)
	}

	return json.Marshal(out)
}
