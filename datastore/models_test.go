package datastore

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"
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

func TestSelfHostedCheckoutAttempt_PreservesNonceInJSON(t *testing.T) {
	attempts := map[string]SelfHostedCheckoutAttempt{
		"attempt_123": {
			AttemptID:         "attempt_123",
			CheckoutNonce:     "raw-nonce",
			CheckoutNonceHash: "nonce-hash",
			Status:            "pending",
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		},
	}

	raw, err := json.Marshal(attempts)
	require.NoError(t, err)
	require.Contains(t, string(raw), "checkout_nonce")

	var decoded map[string]SelfHostedCheckoutAttempt
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Equal(t, "raw-nonce", decoded["attempt_123"].CheckoutNonce)
}

func TestProject_ValidateOutgoingEventIdempotencyKey(t *testing.T) {
	customProject := &Project{
		Type: OutgoingProject,
		Config: &ProjectConfig{
			RequestIDHeader: config.RequestIDHeaderProvider("Split-Request-ID"),
		},
	}

	tests := []struct {
		name           string
		project        *Project
		idempotencyKey string
		wantErr        error
	}{
		{
			name:           "custom_header_requires_idempotency_key",
			project:        customProject,
			idempotencyKey: "",
			wantErr:        ErrMissingIdempotencyKeyForCustomRequestIDHeader,
		},
		{
			name:           "custom_header_with_idempotency_key",
			project:        customProject,
			idempotencyKey: "stable-request-id",
		},
		{
			name:    "default_header_allows_missing_idempotency_key",
			project: &Project{Type: OutgoingProject, Config: &ProjectConfig{}},
		},
		{
			name:    "incoming_project_allows_missing_idempotency_key",
			project: &Project{Type: IncomingProject, Config: &ProjectConfig{RequestIDHeader: "Split-Request-ID"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.project.ValidateOutgoingEventIdempotencyKey(tc.idempotencyKey)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
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

func TestHasArrayWildcardSelector(t *testing.T) {
	tests := []struct {
		name   string
		filter M
		want   bool
	}{
		{
			name:   "matches wildcard path segment",
			filter: M{"items.$.id": "value"},
			want:   true,
		},
		{
			name:   "matches leading wildcard path segment",
			filter: M{"$.id": "value"},
			want:   true,
		},
		{
			name:   "ignores dollar sign within field name",
			filter: M{"price$.amount": "value"},
			want:   false,
		},
		{
			name:   "ignores dollar sign suffix segment",
			filter: M{"price$": "value"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, HasArrayWildcardSelector(tt.filter))
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
