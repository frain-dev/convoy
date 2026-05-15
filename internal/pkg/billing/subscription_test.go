package billing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrgLicenseEntitledBySubscription_ActiveWithPlanID(t *testing.T) {
	require.True(t, OrgLicenseEntitledBySubscription(BillingSubscription{
		Status: "active",
		PlanID: "price_123",
	}))
}

func TestOrgLicenseEntitledBySubscription_ActiveWithNestedPlan(t *testing.T) {
	require.True(t, OrgLicenseEntitledBySubscription(BillingSubscription{
		Status: "active",
		Plan:   &Plan{Name: "Pro"},
	}))
}

func TestOrgLicenseEntitledBySubscription_ActiveWithoutPlan(t *testing.T) {
	require.False(t, OrgLicenseEntitledBySubscription(BillingSubscription{
		Status: "active",
		ID:     "sub_123",
	}))
}

func TestOrgLicenseEntitledBySubscription_Canceled(t *testing.T) {
	require.False(t, OrgLicenseEntitledBySubscription(BillingSubscription{
		Status: "canceled",
		PlanID: "price_123",
	}))
}
