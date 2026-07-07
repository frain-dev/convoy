package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestRedactConfigurationSecrets(t *testing.T) {
	t.Run("strips license and checkout secrets", func(t *testing.T) {
		c := &datastore.Configuration{
			UID:                     "cfg-1",
			IsSignupEnabled:         true,
			LicenseKey:              "live-license-key",
			CheckoutLicenseKey:      "checkout-license-key",
			LicenseKeySource:        "env",
			ActiveCheckoutAttemptID: "att-1",
			CheckoutID:              "co-1",
			ExternalID:              "ext-1",
			CheckoutAttempts: map[string]datastore.SelfHostedCheckoutAttempt{
				"att-1": {AttemptID: "att-1", CheckoutNonce: "secret-nonce"},
			},
		}

		redactConfigurationSecrets(c)

		require.Empty(t, c.LicenseKey)
		require.Empty(t, c.CheckoutLicenseKey)
		require.Empty(t, c.LicenseKeySource)
		require.Empty(t, c.ActiveCheckoutAttemptID)
		require.Empty(t, c.CheckoutID)
		require.Empty(t, c.ExternalID)
		require.Nil(t, c.CheckoutAttempts)

		// Non-sensitive fields the config UI relies on must survive.
		require.Equal(t, "cfg-1", c.UID)
		require.True(t, c.IsSignupEnabled)
	})

	t.Run("nil configuration is a no-op", func(t *testing.T) {
		require.NotPanics(t, func() { redactConfigurationSecrets(nil) })
	})
}
