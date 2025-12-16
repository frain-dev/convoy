package models

import "github.com/frain-dev/convoy/auth"

// Organisation represents the request body for creating/updating an organisation
type Organisation struct {
	Name         string `json:"name" bson:"name"`
	CustomDomain string `json:"custom_domain" bson:"custom_domain"`
}

// OrganisationInvite represents the request body for inviting a user to an organisation
type OrganisationInvite struct {
	InviteeEmail string    `json:"invitee_email" valid:"required~please provide a valid invitee email,email"`
	Role         auth.Role `json:"role" bson:"role"`
}

// UpdateOrganisationMember represents the request body for updating an organisation member's role
type UpdateOrganisationMember struct {
	Role auth.Role `json:"role" bson:"role"`
}

// UpdateOrganisationFeatureFlags represents the request body for updating organisation feature flags
type UpdateOrganisationFeatureFlags struct {
	FeatureFlags map[string]bool `json:"feature_flags" valid:"required"`
}

// UpdateOrganisationOverride represents the request body for updating a feature flag override
type UpdateOrganisationOverride struct {
	FeatureKey string `json:"feature_key" valid:"required"`
	Enabled    bool   `json:"enabled"`
}

// UpdateOrganisationCircuitBreakerConfig represents the request body for updating circuit breaker configuration
type UpdateOrganisationCircuitBreakerConfig struct {
	SampleRate                  uint64 `json:"sample_rate" valid:"required,min(1)"`
	ErrorTimeout                uint64 `json:"error_timeout" valid:"required,min(1)"`
	FailureThreshold            uint64 `json:"failure_threshold" valid:"required,range(0|100)"`
	SuccessThreshold            uint64 `json:"success_threshold" valid:"required,range(0|100)"`
	ObservabilityWindow         uint64 `json:"observability_window" valid:"required,min(1)"`
	MinimumRequestCount         uint64 `json:"minimum_request_count" valid:"required,min(0)"`
	ConsecutiveFailureThreshold uint64 `json:"consecutive_failure_threshold" valid:"required,min(0)"`
}
