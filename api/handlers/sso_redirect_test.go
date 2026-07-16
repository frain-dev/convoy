package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
)

func TestValidateSSOAdminPortalRedirectURL_SelfHostedAcceptsCustomerOrigin(t *testing.T) {
	cfg := config.Configuration{}
	got, err := validateSSOAdminPortalRedirectURL("https://customer.example.com", cfg, "")
	require.NoError(t, err)
	require.Equal(t, "https://customer.example.com", got)
}

func TestValidateSSOAdminPortalRedirectURL_CloudRejectsForeignHost(t *testing.T) {
	cfg := config.Configuration{
		Host: "https://cloud.getconvoy.cloud",
		Billing: config.BillingConfiguration{
			APIKey: "test-api-key",
		},
	}
	_, err := validateSSOAdminPortalRedirectURL("https://evil.example.com", cfg, "https://cloud.getconvoy.cloud")
	require.ErrorIs(t, err, errSSORedirectHostNotApproved)
}

func TestValidateSSOAdminPortalRedirectURL_CloudAcceptsInstanceHost(t *testing.T) {
	cfg := config.Configuration{
		Host: "https://cloud.getconvoy.cloud",
		Billing: config.BillingConfiguration{
			APIKey: "test-api-key",
		},
	}
	got, err := validateSSOAdminPortalRedirectURL("https://cloud.getconvoy.cloud", cfg, "")
	require.NoError(t, err)
	require.Equal(t, "https://cloud.getconvoy.cloud", got)
}

func TestValidateSSOAdminPortalRedirectURL_RejectsPath(t *testing.T) {
	cfg := config.Configuration{}
	_, err := validateSSOAdminPortalRedirectURL("https://customer.example.com/evil", cfg, "")
	require.Error(t, err)
}
