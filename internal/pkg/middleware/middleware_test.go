package middleware

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHeaderFields(t *testing.T) {
	tests := []struct {
		name     string
		input    http.Header
		expected map[string]string
	}{
		{
			name: "redacts authorization header",
			input: http.Header{
				"Authorization": {"Bearer secret-token"},
			},
			expected: map[string]string{"authorization": "***"},
		},
		{
			name: "redacts cookie header",
			input: http.Header{
				"Cookie": {"session=abc123"},
			},
			expected: map[string]string{"cookie": "***"},
		},
		{
			name: "redacts set-cookie header",
			input: http.Header{
				"Set-Cookie": {"session=abc123; HttpOnly"},
			},
			expected: map[string]string{"set-cookie": "***"},
		},
		{
			name: "redacts proxy-authorization header",
			input: http.Header{
				"Proxy-Authorization": {"Basic dXNlcjpwYXNz"},
			},
			expected: map[string]string{"proxy-authorization": "***"},
		},
		{
			name: "redacts x-convoy-signature header",
			input: http.Header{
				"X-Convoy-Signature": {"sha256=abc123deadbeef"},
			},
			expected: map[string]string{"x-convoy-signature": "***"},
		},
		{
			name: "redacts x-hub-signature header",
			input: http.Header{
				"X-Hub-Signature": {"sha1=abc123"},
			},
			expected: map[string]string{"x-hub-signature": "***"},
		},
		{
			name: "redacts x-hub-signature-256 header",
			input: http.Header{
				"X-Hub-Signature-256": {"sha256=abc123"},
			},
			expected: map[string]string{"x-hub-signature-256": "***"},
		},
		{
			name: "redacts x-shopify-hmac-sha256 header",
			input: http.Header{
				"X-Shopify-Hmac-Sha256": {"abc123=="},
			},
			expected: map[string]string{"x-shopify-hmac-sha256": "***"},
		},
		{
			name: "redacts x-twitter-webhooks-signature header",
			input: http.Header{
				"X-Twitter-Webhooks-Signature": {"sha256=abc123"},
			},
			expected: map[string]string{"x-twitter-webhooks-signature": "***"},
		},
		{
			name: "passes non-sensitive single-value header unchanged",
			input: http.Header{
				"Content-Type": {"application/json"},
			},
			expected: map[string]string{"content-type": "application/json"},
		},
		{
			name: "joins non-sensitive multi-value header",
			input: http.Header{
				"Accept": {"application/json", "text/html"},
			},
			expected: map[string]string{"accept": "[application/json], [text/html]"},
		},
		{
			name: "omits header with zero values",
			input: http.Header{
				"X-Empty": {},
			},
			expected: map[string]string{},
		},
		{
			name: "handles mix of sensitive and non-sensitive headers",
			input: http.Header{
				"Authorization": {"Bearer secret"},
				"Content-Type":  {"application/json"},
				"X-Request-Id":  {"req-123"},
			},
			expected: map[string]string{
				"authorization": "***",
				"content-type":  "application/json",
				"x-request-id":  "req-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := headerFields(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSensitiveHeadersNeverStoredRaw(t *testing.T) {
	// Verify that no sensitive header value appears in the output as its raw value.
	rawValue := "super-secret-value-that-must-not-be-logged"

	for name := range sensitiveHeaders {
		h := http.Header{}
		h.Set(name, rawValue)

		result := headerFields(h)

		canonicalKey := http.CanonicalHeaderKey(name)
		resultKey := ""
		for k := range result {
			if http.CanonicalHeaderKey(k) == canonicalKey {
				resultKey = k
				break
			}
		}

		if resultKey != "" {
			assert.NotEqual(t, rawValue, result[resultKey],
				"sensitive header %q must not be logged as raw value", name)
			assert.Equal(t, "***", result[resultKey],
				"sensitive header %q must be redacted to ***", name)
		}
	}
}
