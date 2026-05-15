package billing

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelfHostedLicenseBillingKey_TrimsAndReturns(t *testing.T) {
	t.Parallel()

	k, err := selfHostedLicenseBillingKey("  lk_instance  ")
	require.NoError(t, err)
	require.Equal(t, "lk_instance", k)
}

func TestSelfHostedLicenseBillingKey_ErrNoLicenseWhenEmpty(t *testing.T) {
	t.Parallel()

	k, err := selfHostedLicenseBillingKey("")
	require.ErrorIs(t, err, ErrNoLicense)
	require.Empty(t, k)

	k, err = selfHostedLicenseBillingKey("   ")
	require.ErrorIs(t, err, ErrNoLicense)
	require.Empty(t, k)
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
