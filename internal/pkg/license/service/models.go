package service

import (
	"encoding/json"
	"fmt"
	"time"
)

// ValidateLicenseRequest represents the request to validate a license
type ValidateLicenseRequest struct {
	LicenseKey string `json:"license_key"`
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
