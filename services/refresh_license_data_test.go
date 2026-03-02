package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestCheckOrganisationProjectLimit_NoKey_ReturnsFalseNil(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Billing enabled, mock returns no org license key → resolveKey returns "" → (false, nil).
	mockBilling := &billing.MockBillingClient{}
	// GetOrganisationLicenseKey left empty so Data.Key is ""

	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	// LoadProjects must not be called when key is ""
	// (no EXPECT so any call would fail the test)

	org := &datastore.Organisation{UID: "org-1", LicenseData: ""}
	deps := OrgProjectLimitDeps{
		BillingClient: mockBilling,
		ProjectRepo:   mockProjectRepo,
		Cfg: config.Configuration{
			Billing:    config.BillingConfiguration{Enabled: true},
			LicenseKey: "",
		},
		Logger: log.FromContext(ctx),
	}

	allowed, err := CheckOrganisationProjectLimit(ctx, org, deps)
	require.NoError(t, err)
	require.False(t, allowed)
}
