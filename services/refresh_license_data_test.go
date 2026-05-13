package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/billing"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func TestResolveKeyPrefersInstanceLicenseForLicenseScopedBilling(t *testing.T) {
	org := datastore.Organisation{UID: "org-1"}
	enc, err := license.EncryptLicenseData(org.UID, &license.LicenseDataPayload{Key: "stored-license"})
	require.NoError(t, err)
	org.LicenseData = enc

	cfg := config.Configuration{LicenseKey: "instance-license"}

	key := resolveKey(context.Background(), org, "instance-license", cfg, nil)

	require.Equal(t, "instance-license", key)
}

func TestResolveKeyFallsBackToStoredLicenseWhenInstanceLicenseIsEmpty(t *testing.T) {
	org := datastore.Organisation{UID: "org-1"}
	enc, err := license.EncryptLicenseData(org.UID, &license.LicenseDataPayload{Key: "stored-license"})
	require.NoError(t, err)
	org.LicenseData = enc

	cfg := config.Configuration{}

	key := resolveKey(context.Background(), org, "", cfg, nil)

	require.Equal(t, "stored-license", key)
}

func TestCheckOrganisationProjectLimit_NoKey_ReturnsTrueNil(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockBilling := &billing.MockBillingClient{}

	mockProjectRepo := mocks.NewMockProjectRepository(ctrl)
	// LoadProjects must not be called when key is ""
	// (no EXPECT so any call would fail the test)

	org := &datastore.Organisation{UID: "org-1", LicenseData: ""}
	deps := OrgProjectLimitDeps{
		BillingClient: mockBilling,
		ProjectRepo:   mockProjectRepo,
		Cfg: config.Configuration{
			Billing:    config.BillingConfiguration{URL: "http://billing.test"},
			LicenseKey: "",
		},
		Logger: log.New("convoy", log.LevelInfo),
	}

	allowed, err := CheckOrganisationProjectLimit(ctx, org, deps)
	require.NoError(t, err)
	require.True(t, allowed)
}
