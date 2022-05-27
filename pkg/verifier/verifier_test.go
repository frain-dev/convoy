package verifier

import (
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_HmacVerifier_VerifyRequest(t *testing.T) {
	tests := map[string]struct {
		opts          map[string]string
		payload       []byte
		requestFn     func(t *testing.T) *http.Request
		expectedError error
	}{
		"invalid_signature": {
			opts: map[string]string{
				"header":   "X-Convoy-Signature",
				"hash":     "SHA512",
				"secret":   "Convoy",
				"encoding": "hex",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				hash := hex.EncodeToString([]byte(`Obviously wrong hash`))

				req.Header.Add("X-Convoy-Signature", hash)
				return req
			},
			expectedError: ErrHashDoesNotMatch,
		},
		"invalid_hex_encoding": {
			opts: map[string]string{
				"header":   "X-Convoy-Signature",
				"hash":     "SHA512",
				"secret":   "Convoy",
				"encoding": "hex",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				hash := "Hash with characters outside hex"

				req.Header.Add("X-Convoy-Signature", hash)
				return req
			},
			expectedError: ErrCannotDecodeHexEncodedMACHeader,
		},
		"invalid_base64_encoding": {
			opts: map[string]string{
				"header":   "X-Convoy-Signature",
				"hash":     "SHA512",
				"secret":   "Convoy",
				"encoding": "base64",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				hash := "Hash with characters outside %^&$*#@ base64"

				req.Header.Add("X-Convoy-Signature", hash)
				return req
			},
			expectedError: ErrCannotDecodeBase64EncodedMACHeader,
		},
		"empty_signature": {
			opts: map[string]string{
				"header":   "X-Convoy-Signature",
				"hash":     "SHA512",
				"secret":   "Convoy",
				"encoding": "base64",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				req.Header.Add("X-Convoy-Signature", "")
				return req
			},
			expectedError: ErrSignatureCannotBeEmpty,
		},
		"valid_hex_request": {
			opts: map[string]string{
				"header":   "X-Convoy-Signature",
				"hash":     "SHA512",
				"secret":   "Convoy",
				"encoding": "hex",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				hash := "83306382f5361d35351d6de45998f23b52f40bcf96befe4e92f137c0f1" +
					"bf4a7119388b238d8f9d502ac77e6f1a8849a4778272667ed88d530cac8050bd1fee2d"

				req.Header.Add("X-Convoy-Signature", hash)
				return req
			},
			expectedError: nil,
		},
		"valid_base64_request": {
			opts: map[string]string{
				"header":   "X-Convoy-Signature",
				"hash":     "SHA512",
				"secret":   "Convoy",
				"encoding": "base64",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				hash := "gzBjgvU2HTU1HW3kWZjyO1L0C8+Wvv5OkvE3wPG/S" +
					"nEZOIsjjY+dUCrHfm8aiEmkd4JyZn7YjVMMrIBQvR/uLQ=="

				req.Header.Add("X-Convoy-Signature", hash)
				return req
			},
			expectedError: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange.
			v := NewHmacVerifier(
				tc.opts["header"],
				tc.opts["hash"],
				tc.opts["secret"],
				tc.opts["encoding"])
			req := tc.requestFn(t)

			// Assert.
			err := v.VerifyRequest(req, tc.payload)

			// Act.
			require.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func Test_BasicAuthVerifier_VerifyRequest(t *testing.T) {
	tests := map[string]struct {
		opts          map[string]string
		payload       []byte
		requestFn     func(t *testing.T, c map[string]string) *http.Request
		expectedError error
	}{
		"valid_request": {
			opts: map[string]string{
				"username": "convoy-ingester",
				"password": "convoy-ingester",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T, c map[string]string) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				req.SetBasicAuth(c["username"], c["password"])

				return req
			},
			expectedError: nil,
		},
		"wrong_credentials": {
			opts: map[string]string{
				"username": "convoy-ingester",
				"password": "convoy-ingester",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T, c map[string]string) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				req.SetBasicAuth("wrong-username", "wrong-password")

				return req
			},
			expectedError: ErrAuthHeader,
		},
		"invalid_credentials": {
			opts: map[string]string{
				"username": "convoy-ingester",
				"password": "convoy-ingester",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T, c map[string]string) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				req.Header.Add("Authorization", "Basic wrongbase64str")

				return req
			},
			expectedError: ErrInvalidHeaderStructure,
		},
		"invalid_header_format": {
			opts: map[string]string{
				"username": "convoy-ingester",
				"password": "convoy-password",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T, c map[string]string) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				req.Header.Add("Authorization", "Basic")

				return req
			},
			expectedError: ErrInvalidHeaderStructure,
		},
		"empty_auth_header": {
			opts: map[string]string{
				"username": "convoy-ingester",
				"password": "convoy-password",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T, c map[string]string) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				return req
			},
			expectedError: ErrInvalidHeaderStructure,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange
			v := NewBasicAuthVerifier(tc.opts["username"], tc.opts["password"])
			req := tc.requestFn(t, tc.opts)

			// Assert
			err := v.VerifyRequest(req, tc.payload)

			// Act.
			require.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func Test_APIKeyVerifier_VerifyRequest(t *testing.T) {
	tests := map[string]struct {
		opts          map[string]string
		payload       []byte
		requestFn     func(t *testing.T, c map[string]string) *http.Request
		expectedError error
	}{
		"invalid_api_key": {
			opts: map[string]string{
				"key":    "sec_apikeysecret",
				"header": "Authorization",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T, c map[string]string) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				req.Header.Add("Authorization", "Bearer sec_invalidkey")
				return req
			},
			expectedError: ErrAuthHeader,
		},
		"valid_request": {
			opts: map[string]string{
				"key":    "sec_apikeysecret",
				"header": "webhook-key",
			},
			payload: []byte(`Test Payload Body`),
			requestFn: func(t *testing.T, c map[string]string) *http.Request {
				req, err := http.NewRequest("POST", "URL", strings.NewReader(``))
				require.NoError(t, err)

				req.Header.Add(c["header"], c["key"])
				return req
			},
			expectedError: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange
			v := NewAPIKeyVerifier(tc.opts["key"], tc.opts["header"])
			req := tc.requestFn(t, tc.opts)

			// Assert
			err := v.VerifyRequest(req, tc.payload)

			// Act.
			require.ErrorIs(t, err, tc.expectedError)
		})
	}
}
