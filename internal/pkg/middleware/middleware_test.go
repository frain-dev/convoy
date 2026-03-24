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
			name: "redacts unknown custom header",
			input: http.Header{
				"X-Custom-Header": {"some-value"},
			},
			expected: map[string]string{"x-custom-header": "***"},
		},
		{
			name: "passes content-type header unchanged",
			input: http.Header{
				"Content-Type": {"application/json"},
			},
			expected: map[string]string{"content-type": "application/json"},
		},
		{
			name: "passes user-agent header unchanged",
			input: http.Header{
				"User-Agent": {"convoy/1.0"},
			},
			expected: map[string]string{"user-agent": "convoy/1.0"},
		},
		{
			name: "passes x-forwarded-for header unchanged",
			input: http.Header{
				"X-Forwarded-For": {"192.168.1.1"},
			},
			expected: map[string]string{"x-forwarded-for": "192.168.1.1"},
		},
		{
			name: "joins multi-value safe header",
			input: http.Header{
				"Accept": {"application/json", "text/html"},
			},
			expected: map[string]string{"accept": "[application/json], [text/html]"},
		},
		{
			name: "omits header with zero values",
			input: http.Header{
				"Content-Type": {},
			},
			expected: map[string]string{},
		},
		{
			name: "handles mix of safe and sensitive headers",
			input: http.Header{
				"Authorization": {"Bearer secret"},
				"Content-Type":  {"application/json"},
				"User-Agent":    {"convoy/1.0"},
			},
			expected: map[string]string{
				"authorization": "***",
				"content-type":  "application/json",
				"user-agent":    "convoy/1.0",
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

func TestSensitiveHeaderValuesNeverLogged(t *testing.T) {
	sensitiveNames := []string{
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"Set-Cookie",
		"X-Convoy-Signature",
		"X-Hub-Signature",
		"X-Hub-Signature-256",
		"X-Shopify-Hmac-Sha256",
		"X-Twitter-Webhooks-Signature",
	}

	rawValue := "super-secret-value-that-must-not-be-logged"

	for _, name := range sensitiveNames {
		h := http.Header{}
		h.Set(name, rawValue)

		result := headerFields(h)

		for _, v := range result {
			assert.NotEqual(t, rawValue, v,
				"sensitive header %q raw value must not appear in log output", name)
		}
	}
}
