package license

import (
	"encoding/json"

	"github.com/frain-dev/convoy/internal/pkg/license/service"
)

var featureKeys = []struct {
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

func buildFeatureListFromEntitlements(parsed map[string]service.EntitlementValue, orgProjectCount int64) map[string]interface{} {
	out := make(map[string]interface{})
	for _, key := range []string{"org_limit", "user_limit", "project_limit"} {
		limit, exists := service.GetNumberEntitlement(parsed, key)
		if exists {
			available := limit > 0 || limit == -1
			current := int64(0)
			limitReached := false
			if key == "project_limit" && orgProjectCount >= 0 {
				current = orgProjectCount
				limitReached = available && limit >= 0 && orgProjectCount >= limit
			}
			out[key] = map[string]interface{}{
				"limit":         limit,
				"allowed":       !limitReached,
				"current":       current,
				"available":     available,
				"limit_reached": limitReached,
			}
		}
	}
	for _, f := range featureKeys {
		out[f.outputKey] = service.GetBoolEntitlement(parsed, f.entitlementKey)
	}
	return out
}

func FeatureListFromEntitlements(entitlements map[string]interface{}) (json.RawMessage, error) {
	if len(entitlements) == 0 {
		return json.Marshal(map[string]interface{}{})
	}
	return json.Marshal(buildFeatureListFromEntitlements(service.ParseEntitlements(entitlements), -1))
}

func BillingRequiredFeatureListJSON() (json.RawMessage, error) {
	limitBlock := map[string]interface{}{
		"limit": 0, "allowed": false, "current": 0, "available": false, "limit_reached": true,
	}
	out := map[string]interface{}{
		"org_limit": limitBlock, "user_limit": limitBlock, "project_limit": limitBlock,
	}
	for _, f := range featureKeys {
		out[f.outputKey] = false
	}
	return json.Marshal(out)
}

func FeatureListFromEntitlementsWithOrgProjectCount(entitlements map[string]interface{}, projectCount int64) (json.RawMessage, error) {
	if len(entitlements) == 0 {
		return json.Marshal(map[string]interface{}{})
	}
	return json.Marshal(buildFeatureListFromEntitlements(service.ParseEntitlements(entitlements), projectCount))
}
