package billing

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLicensedStrategy_LicenseKeyFor_AllowsInstanceScopeWithoutOrg(t *testing.T) {
	t.Parallel()

	s := &licensedStrategy{
		client:             &MockBillingClient{},
		instanceLicenseKey: "lk_instance",
	}

	key, err := s.licenseKeyFor(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, "lk_instance", key)
}

func TestLicensedStrategy_LicenseKeyFor_BlocksMismatchedOrg(t *testing.T) {
	t.Parallel()

	s := &licensedStrategy{
		client:             &MockBillingClient{},
		instanceLicenseKey: "lk_instance",
	}

	_, err := s.licenseKeyFor(context.Background(), "org-mismatch")
	require.Error(t, err)

	var serviceErr *ServiceError
	require.True(t, errors.As(err, &serviceErr))
	require.Equal(t, http.StatusForbidden, serviceErr.StatusCode)
}

func TestLicensedStrategy_LicenseKeyFor_AllowsBoundOrg(t *testing.T) {
	t.Parallel()

	s := &licensedStrategy{
		client:             &MockBillingClient{},
		instanceLicenseKey: "lk_instance",
	}

	key, err := s.licenseKeyFor(context.Background(), "ext")
	require.NoError(t, err)
	require.Equal(t, "lk_instance", key)
}
