package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

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

	t.Run("strips S3 storage credentials, keeps location metadata", func(t *testing.T) {
		c := &datastore.Configuration{
			StoragePolicy: &datastore.StoragePolicyConfiguration{
				Type: datastore.S3,
				S3: &datastore.S3Storage{
					Bucket:       null.StringFrom("my-bucket"),
					Region:       null.StringFrom("us-east-1"),
					Endpoint:     null.StringFrom("https://s3.example.com"),
					AccessKey:    null.StringFrom("AKIA-secret"),
					SecretKey:    null.StringFrom("super-secret"),
					SessionToken: null.StringFrom("session-secret"),
				},
			},
		}

		redactConfigurationSecrets(c)

		require.Empty(t, c.StoragePolicy.S3.AccessKey.String)
		require.Empty(t, c.StoragePolicy.S3.SecretKey.String)
		require.Empty(t, c.StoragePolicy.S3.SessionToken.String)
		require.Equal(t, "my-bucket", c.StoragePolicy.S3.Bucket.String)
		require.Equal(t, "us-east-1", c.StoragePolicy.S3.Region.String)
		require.Equal(t, "https://s3.example.com", c.StoragePolicy.S3.Endpoint.String)
	})

	t.Run("strips Azure account key, keeps location metadata", func(t *testing.T) {
		c := &datastore.Configuration{
			StoragePolicy: &datastore.StoragePolicyConfiguration{
				Type: datastore.AzureBlob,
				AzureBlob: &datastore.AzureBlobStorage{
					AccountName:   null.StringFrom("acct"),
					AccountKey:    null.StringFrom("azure-secret-key"),
					ContainerName: null.StringFrom("container"),
					Endpoint:      null.StringFrom("https://acct.blob.core.windows.net"),
				},
			},
		}

		redactConfigurationSecrets(c)

		require.Empty(t, c.StoragePolicy.AzureBlob.AccountKey.String)
		require.Equal(t, "acct", c.StoragePolicy.AzureBlob.AccountName.String)
		require.Equal(t, "container", c.StoragePolicy.AzureBlob.ContainerName.String)
	})

	t.Run("strips on-prem path", func(t *testing.T) {
		c := &datastore.Configuration{
			StoragePolicy: &datastore.StoragePolicyConfiguration{
				Type:   datastore.OnPrem,
				OnPrem: &datastore.OnPremStorage{Path: null.StringFrom("/var/lib/convoy/backups")},
			},
		}

		redactConfigurationSecrets(c)

		require.Empty(t, c.StoragePolicy.OnPrem.Path.String)
	})

	t.Run("nil configuration is a no-op", func(t *testing.T) {
		require.NotPanics(t, func() { redactConfigurationSecrets(nil) })
	})

	t.Run("nil storage policy is a no-op", func(t *testing.T) {
		c := &datastore.Configuration{UID: "cfg-2"}
		require.NotPanics(t, func() { redactConfigurationSecrets(c) })
	})
}
