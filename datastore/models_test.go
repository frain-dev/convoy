package datastore

import (
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/stretchr/testify/require"
)

func TestProject_IsDeleted(t *testing.T) {
	d := null.NewTime(time.Unix(39487, 0), true)
	deletedAt := null.NewTime(time.Now(), true)

	tt := []struct {
		name      string
		project   *Project
		isDeleted bool
	}{
		{
			name:    "set deleted_at to zero",
			project: &Project{UID: "123456", DeletedAt: null.NewTime(time.Now(), false)},
		},
		{
			name:    "skip deleted_at field",
			project: &Project{UID: "123456"},
		},
		{
			name:      "set deleted_at to random integer",
			project:   &Project{UID: "123456", DeletedAt: d},
			isDeleted: true,
		},
		{
			name:      "set deleted_at to current timestamp",
			project:   &Project{UID: "123456", DeletedAt: deletedAt},
			isDeleted: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isDeleted, tc.project.IsDeleted())
		})
	}
}

func TestProject_IsOwner(t *testing.T) {
	tt := []struct {
		name     string
		project  *Project
		endpoint *Endpoint
		isOwner  bool
	}{
		{
			name:     "right owner",
			project:  &Project{UID: "123456", DeletedAt: null.NewTime(time.Now(), false)},
			endpoint: &Endpoint{ProjectID: "123456"},
			isOwner:  true,
		},
		{
			name:     "wrong owner",
			project:  &Project{UID: "123456", DeletedAt: null.NewTime(time.Now(), false)},
			endpoint: &Endpoint{ProjectID: "1234567"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isOwner, tc.project.IsOwner(tc.endpoint))
		})
	}
}

func TestMtlsClientCert_ScanAndValue(t *testing.T) {
	t.Run("should scan valid JSON", func(t *testing.T) {
		jsonData := []byte(`{"client_cert":"-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----","client_key":"-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----"}`)

		var mtls MtlsClientCert
		err := mtls.Scan(jsonData)

		require.NoError(t, err)
		require.Equal(t, "-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----", mtls.ClientCert)
		require.Equal(t, "-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----", mtls.ClientKey)
	})

	t.Run("should handle null value", func(t *testing.T) {
		var mtls MtlsClientCert
		err := mtls.Scan(nil)

		require.NoError(t, err)
	})

	t.Run("should handle null string", func(t *testing.T) {
		var mtls MtlsClientCert
		err := mtls.Scan([]byte("null"))

		require.NoError(t, err)
	})

	t.Run("should return error for invalid type", func(t *testing.T) {
		var mtls MtlsClientCert
		err := mtls.Scan(123)

		require.Error(t, err)
		require.Contains(t, err.Error(), "unsupported value type")
	})

	t.Run("should marshal to JSON", func(t *testing.T) {
		mtls := MtlsClientCert{
			ClientCert: "-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----",
			ClientKey:  "-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----",
		}

		val, err := mtls.Value()

		require.NoError(t, err)
		require.NotNil(t, val)

		jsonBytes, ok := val.([]byte)
		require.True(t, ok)
		require.Contains(t, string(jsonBytes), "client_cert")
		require.Contains(t, string(jsonBytes), "test-cert")
	})

	t.Run("should return nil for empty struct", func(t *testing.T) {
		mtls := MtlsClientCert{}

		val, err := mtls.Value()

		require.NoError(t, err)
		require.Nil(t, val)
	})

	t.Run("should roundtrip correctly", func(t *testing.T) {
		original := MtlsClientCert{
			ClientCert: "-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----",
			ClientKey:  "-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----",
		}

		// Marshal
		val, err := original.Value()
		require.NoError(t, err)

		// Unmarshal
		var decoded MtlsClientCert
		err = decoded.Scan(val)
		require.NoError(t, err)

		require.Equal(t, original.ClientCert, decoded.ClientCert)
		require.Equal(t, original.ClientKey, decoded.ClientKey)
	})
}
