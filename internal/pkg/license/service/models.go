package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// UsageSnapshot is anonymized instance-wide counts attached to license validate.
// Counts only: no hostnames, URLs, emails, or org names.
type UsageSnapshot struct {
	EndpointCount int64  `json:"endpoint_count"`
	EventCount    int64  `json:"event_count"`
	ProjectCount  int64  `json:"project_count"`
	OrgCount      int64  `json:"org_count"`
	UserCount     int64  `json:"user_count"`
	AsOf          string `json:"as_of,omitempty"` // RFC3339
}

// ValidateLicenseRequest represents the request to validate a license.
// Version, DeploymentID, and Usage are optional and omitted when empty/nil.
type ValidateLicenseRequest struct {
	LicenseKey   string         `json:"license_key"`
	Version      string         `json:"version,omitempty"`
	DeploymentID string         `json:"deployment_id,omitempty"`
	Usage        *UsageSnapshot `json:"usage,omitempty"`
}

// UsageLoader loads a cached usage snapshot for license validate (fail open).
type UsageLoader interface {
	LoadCached(ctx context.Context) (*UsageSnapshot, error)
}

// LicenseValidationResponse represents the response from the license service
type LicenseValidationResponse struct {
	Status  bool                   `json:"status"`
	Message string                 `json:"message"`
	Data    *LicenseValidationData `json:"data,omitempty"`
}

// LicenseValidationData contains the license validation details
type LicenseValidationData struct {
	Valid        bool            `json:"valid"`
	Status       string          `json:"status"`       // active, suspended, expired, revoked
	Entitlements json.RawMessage `json:"entitlements"` // Array format: [{"key": "...", "value": ...}]
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
}

// EntitlementItem represents an entitlement in array format
type EntitlementItem struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// GetEntitlementsMap converts entitlements from array format to a map
func (d *LicenseValidationData) GetEntitlementsMap() (map[string]interface{}, error) {
	if len(d.Entitlements) == 0 {
		return make(map[string]interface{}), nil
	}

	var arrayEntitlements []EntitlementItem
	if err := json.Unmarshal(d.Entitlements, &arrayEntitlements); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entitlements array: %w", err)
	}

	result := make(map[string]interface{})
	for _, item := range arrayEntitlements {
		result[item.Key] = item.Value
	}

	return result, nil
}

// License errors
var (
	ErrLicenseNotFound   = &LicenseError{Message: "License not found", Status: "not_found"}
	ErrLicenseSuspended  = &LicenseError{Message: "License is suspended", Status: "suspended"}
	ErrLicenseRevoked    = &LicenseError{Message: "License has been revoked", Status: "revoked"}
	ErrLicenseValidation = &LicenseError{Message: "License validation failed", Status: "validation_failed"}
)

// LicenseError represents an error from license validation
type LicenseError struct {
	Message string
	Status  string
}

func (e *LicenseError) Error() string {
	return e.Message
}
