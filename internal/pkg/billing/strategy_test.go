package billing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/mocks"
)

func TestResolveOrgLicenseKey_UsesInstanceKeyWhenOrgIDMissing(t *testing.T) {
	t.Parallel()

	licenseKey, err := resolveOrgLicenseKey(context.Background(), nil, "  lk_instance  ", "")
	require.NoError(t, err)
	require.Equal(t, "lk_instance", licenseKey)
}

func TestResolveOrgLicenseKey_UsesOrganisationKeyWhenOrgIDProvided(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-123"
	orgLicenseKey := "lk_org_123"
	encryptedLicenseData, err := license.EncryptLicenseData(orgID, &license.LicenseDataPayload{Key: orgLicenseKey})
	require.NoError(t, err)

	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	orgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID, LicenseData: encryptedLicenseData}, nil)

	licenseKey, err := resolveOrgLicenseKey(context.Background(), orgRepo, "lk_instance", orgID)
	require.NoError(t, err)
	require.Equal(t, orgLicenseKey, licenseKey)
}

func TestResolveOrgLicenseKey_DoesNotFallbackToInstanceKeyForOrgScopedCalls(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orgID := "org-without-license"
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	orgRepo.EXPECT().
		FetchOrganisationByID(gomock.Any(), orgID).
		Return(&datastore.Organisation{UID: orgID}, nil)

	licenseKey, err := resolveOrgLicenseKey(context.Background(), orgRepo, "lk_instance", orgID)
	require.ErrorIs(t, err, ErrNoLicense)
	require.Empty(t, licenseKey)
}

func TestMonthBoundsUTCUsesUTC(t *testing.T) {
	loc := time.FixedZone("WAT", 60*60)
	now := time.Date(2026, time.May, 13, 15, 30, 0, 0, loc)

	start, end := monthBoundsUTC(now)

	require.Equal(t, time.UTC, start.Location())
	require.Equal(t, time.UTC, end.Location())
	require.Equal(t, time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond), end)
}
