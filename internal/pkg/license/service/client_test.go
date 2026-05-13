package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckLicenseStatus(t *testing.T) {
	client := &Client{}

	require.NoError(t, client.checkLicenseStatus("active"))
	require.NoError(t, client.checkLicenseStatus("pending_payment"))
	require.ErrorIs(t, client.checkLicenseStatus("suspended"), ErrLicenseSuspended)
	require.ErrorIs(t, client.checkLicenseStatus("expired"), ErrLicenseExpired)
	require.ErrorIs(t, client.checkLicenseStatus("revoked"), ErrLicenseRevoked)
	require.EqualError(t, client.checkLicenseStatus("unknown"), "unknown license status: unknown")
}
