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
	{"basic_auth_endpoint_auth", "BasicAuthEndpointAuth"},
	{"endpoint_url_templates", "EndpointURLTemplates"},
	{"use_forward_proxy", "UseForwardProxy"},
	{"asynq_monitoring", "AsynqMonitoring"},
	{"agent_execution_mode", "AgentExecutionMode"},
}

// entitlementUsage carries resolved usage counts for the org-scoped feature list.
// A -1 count means "unknown": don't gate (current stays 0, limit_reached stays false).
type entitlementUsage struct {
	orgCount     int64
	memberCount  int64
	projectCount int64
}

func buildFeatureListFromEntitlements(parsed map[string]service.EntitlementValue, usage entitlementUsage) map[string]interface{} {
	out := make(map[string]interface{})
	for _, key := range []string{"org_limit", "user_limit", "project_limit"} {
		limit, exists := service.GetNumberEntitlement(parsed, key)
		if exists {
			available := limit > 0 || limit == -1
			current := int64(0)
			limitReached := false
			switch key {
			case "project_limit":
				if usage.projectCount >= 0 {
					current = usage.projectCount
					limitReached = available && limit >= 0 && usage.projectCount >= limit
				}
			case "org_limit":
				if usage.orgCount >= 0 {
					current = usage.orgCount
					limitReached = available && limit >= 0 && usage.orgCount >= limit
				}
			case "user_limit":
				if usage.memberCount >= 0 {
					current = usage.memberCount
					limitReached = available && limit >= 0 && usage.memberCount >= limit
				}
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
	return json.Marshal(buildFeatureListFromEntitlements(service.ParseEntitlements(entitlements), entitlementUsage{orgCount: -1, memberCount: -1, projectCount: -1}))
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

// FeatureListFromEntitlementsWithUsage builds the org-scoped feature list with resolved
// usage counts so the UI can gate the add-org, add-member, and add-project actions. Pass
// -1 for any count that cannot be resolved (e.g. no authed user for orgCount on portal
// routes) to keep that limit fail-open (never gated).
func FeatureListFromEntitlementsWithUsage(entitlements map[string]interface{}, orgCount, memberCount, projectCount int64) (json.RawMessage, error) {
	if len(entitlements) == 0 {
		return json.Marshal(map[string]interface{}{})
	}
	return json.Marshal(buildFeatureListFromEntitlements(service.ParseEntitlements(entitlements), entitlementUsage{orgCount: orgCount, memberCount: memberCount, projectCount: projectCount}))
}

// OrgEntitlementCap decrypts an organisation's own license_data and returns the finite
// per-org cap for key and whether that cap must be enforced. It is the single source of
// truth for cloud per-org limits (org_limit, user_limit), matching the trial event cap
// and the display builder; it never falls back to the instance/platform license.
//
// applies is false (caller must fail OPEN, i.e. not gate) when there is no finite cap:
// empty or unreadable license_data, an absent entitlement, or an unlimited (-1) / non-
// positive value. applies is true only for a resolved finite cap (> 0), where the caller
// fails CLOSED once the current count reaches limit.
func OrgEntitlementCap(orgID, licenseData, key string) (limit int64, applies bool) {
	if licenseData == "" {
		return 0, false
	}
	payload, err := DecryptLicenseData(orgID, licenseData)
	if err != nil || payload == nil {
		return 0, false
	}
	entitlements := service.ParseEntitlements(payload.Entitlements)
	limit, ok := service.GetNumberEntitlement(entitlements, key)
	if !ok || limit <= 0 {
		return 0, false
	}
	return limit, true
}
