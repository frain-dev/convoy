package service

import (
	"fmt"
	"strconv"
	"strings"
)

// EntitlementValue represents a value from entitlements (can be bool, number, or string)
type EntitlementValue interface{}

// ParseEntitlements parses entitlements from the license service response
// Entitlements can come in two formats:
// 1. Map format: {"enterprise_sso": true, "user_limit": 25}
// 2. Array format: [{"key": "enterprise_sso", "value": true}, {"key": "user_limit", "value": 25}]
func ParseEntitlements(entitlements map[string]interface{}) map[string]EntitlementValue {
	result := make(map[string]EntitlementValue)

	for key, value := range entitlements {
		result[key] = value
	}

	return result
}

// GetBoolEntitlement retrieves a boolean entitlement value
func GetBoolEntitlement(entitlements map[string]EntitlementValue, key string) bool {
	val, ok := entitlements[key]
	if !ok {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	default:
		return false
	}
}

// GetNumberEntitlement retrieves a number entitlement value
// Returns the value and a boolean indicating if it exists
// -1 typically means unlimited
func GetNumberEntitlement(entitlements map[string]EntitlementValue, key string) (int64, bool) {
	val, ok := entitlements[key]
	if !ok {
		return 0, false
	}

	switch v := val.(type) {
	case float64:
		return int64(v), true
	case int:
		return int64(v), true
	case int64:
		return v, true
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// MapEntitlementKeyToFeature converts snake_case entitlement keys to feature names
// Example: "enterprise_sso" -> "EnterpriseSSO"
func MapEntitlementKeyToFeature(key string) string {
	parts := strings.Split(key, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		}
	}
	return strings.Join(parts, "")
}

// EntitlementKeyMapping maps entitlement keys from license service to Convoy feature methods
var EntitlementKeyMapping = map[string]string{
	"enterprise_sso":               "EnterpriseSSO",
	"portal_links":                 "PortalLinks",
	"webhook_transformations":      "Transformations",
	"advanced_subscriptions":       "AdvancedSubscriptions",
	"webhook_analytics":            "WebhookAnalytics",
	"advanced_webhook_filtering":   "AdvancedWebhookFiltering",
	"advanced_endpoint_mgmt":       "AdvancedEndpointMgmt",
	"webhook_archiving":            "WebhookArchiving",
	"circuit_breaking":             "CircuitBreaking",
	"consumer_pool_tuning":         "ConsumerPoolTuning",
	"google_oauth":                 "GoogleOAuth",
	"export_prometheus_metrics":    "CanExportPrometheusMetrics",
	"read_replica":                 "ReadReplica",
	"credential_encryption":        "CredentialEncryption",
	"ip_rules":                     "IpRules",
	"rbac":                         "RBAC", // Note: RBAC might not be in Licenser interface yet
	"retention_policy":             "RetentionPolicy",
	"mutual_tls":                   "MutualTLS",
	"datadog_tracing":              "DatadogTracing",
	"custom_certificate_authority": "CustomCertificateAuthority",
	"static_ip":                    "StaticIP",
	"oauth2_endpoint_auth":         "OAuth2EndpointAuth",
	"use_forward_proxy":            "UseForwardProxy",
	"asynq_monitoring":             "AsynqMonitoring",
	"agent_execution_mode":         "AgentExecutionMode",
}

// LimitEntitlementMapping maps limit entitlement keys (deprecated CreateOrg/CreateUser/CreateProject removed)
var LimitEntitlementMapping = map[string]string{
	"ingest_rate_limit": "IngestRate",
}

// GetFeatureFromEntitlementKey returns the feature method name for an entitlement key
func GetFeatureFromEntitlementKey(key string) (string, bool) {
	// Check direct mapping first
	if feature, ok := EntitlementKeyMapping[key]; ok {
		return feature, true
	}

	// Check limit mapping
	if feature, ok := LimitEntitlementMapping[key]; ok {
		return feature, true
	}

	// Try to convert snake_case to PascalCase
	feature := MapEntitlementKeyToFeature(key)
	return feature, false // false indicates it's a best-guess conversion
}

// ValidateEntitlementFormat checks if entitlements are in expected format
func ValidateEntitlementFormat(entitlements map[string]interface{}) error {
	if entitlements == nil {
		return fmt.Errorf("entitlements cannot be nil")
	}
	return nil
}
