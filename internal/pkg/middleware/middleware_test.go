package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
)

func TestHeaderFields(t *testing.T) {
	tests := []struct {
		name     string
		input    http.Header
		expected map[string]string
	}{
		// --- explicit sensitiveHeaders ---
		{
			name:     "redacts authorization",
			input:    http.Header{"Authorization": {"Bearer secret-token"}},
			expected: map[string]string{"authorization": "***"},
		},
		{
			name:     "redacts cookie",
			input:    http.Header{"Cookie": {"session=abc123"}},
			expected: map[string]string{"cookie": "***"},
		},
		{
			name:     "redacts set-cookie",
			input:    http.Header{"Set-Cookie": {"session=abc123; HttpOnly"}},
			expected: map[string]string{"set-cookie": "***"},
		},
		{
			name:     "redacts proxy-authorization",
			input:    http.Header{"Proxy-Authorization": {"Basic dXNlcjpwYXNz"}},
			expected: map[string]string{"proxy-authorization": "***"},
		},
		{
			name:     "redacts x-forwarded-for",
			input:    http.Header{"X-Forwarded-For": {"192.168.1.1"}},
			expected: map[string]string{"x-forwarded-for": "***"},
		},
		{
			name:     "redacts x-real-ip",
			input:    http.Header{"X-Real-Ip": {"10.0.0.1"}},
			expected: map[string]string{"x-real-ip": "***"},
		},
		{
			name:     "redacts x-api-key",
			input:    http.Header{"X-Api-Key": {"key-abc123"}},
			expected: map[string]string{"x-api-key": "***"},
		},
		{
			name:     "redacts x-auth-token",
			input:    http.Header{"X-Auth-Token": {"tok-abc123"}},
			expected: map[string]string{"x-auth-token": "***"},
		},
		// --- sensitivePatterns suffix matching ---
		{
			name:     "redacts header ending in -signature",
			input:    http.Header{"X-Hub-Signature": {"sha1=abc"}},
			expected: map[string]string{"x-hub-signature": "***"},
		},
		{
			name:     "redacts header ending in -secret",
			input:    http.Header{"X-Webhook-Secret": {"s3cr3t"}},
			expected: map[string]string{"x-webhook-secret": "***"},
		},
		{
			name:     "redacts header ending in -token",
			input:    http.Header{"X-Access-Token": {"tok123"}},
			expected: map[string]string{"x-access-token": "***"},
		},
		{
			name:     "redacts header ending in -key",
			input:    http.Header{"X-Encryption-Key": {"key123"}},
			expected: map[string]string{"x-encryption-key": "***"},
		},
		{
			name:     "redacts header ending in -password",
			input:    http.Header{"X-User-Password": {"hunter2"}},
			expected: map[string]string{"x-user-password": "***"},
		},
		{
			name:     "redacts header ending in -credential",
			input:    http.Header{"X-Service-Credential": {"cred123"}},
			expected: map[string]string{"x-service-credential": "***"},
		},
		// --- not in safeHeaders → redacted ---
		{
			name:     "redacts unknown custom header",
			input:    http.Header{"X-Custom-Header": {"some-value"}},
			expected: map[string]string{"x-custom-header": "***"},
		},
		// --- safeHeaders → passed through ---
		{
			name:     "passes content-type unchanged",
			input:    http.Header{"Content-Type": {"application/json"}},
			expected: map[string]string{"content-type": "application/json"},
		},
		{
			name:     "passes user-agent unchanged",
			input:    http.Header{"User-Agent": {"convoy/1.0"}},
			expected: map[string]string{"user-agent": "convoy/1.0"},
		},
		{
			name:     "passes x-request-id unchanged",
			input:    http.Header{"X-Request-Id": {"req-123"}},
			expected: map[string]string{"x-request-id": "req-123"},
		},
		{
			name:     "redacts idempotency-key (matches -key pattern)",
			input:    http.Header{"Idempotency-Key": {"idem-abc"}},
			expected: map[string]string{"idempotency-key": "***"},
		},
		// --- multi-value and edge cases ---
		{
			name:  "joins multi-value safe header",
			input: http.Header{"Accept": {"application/json", "text/html"}},
			expected: map[string]string{
				"accept": "[application/json], [text/html]",
			},
		},
		{
			name:     "omits header with zero values",
			input:    http.Header{"Content-Type": {}},
			expected: map[string]string{},
		},
		{
			name: "handles mix of safe, sensitive, and unknown headers",
			input: http.Header{
				"Authorization": {"Bearer secret"},
				"Content-Type":  {"application/json"},
				"User-Agent":    {"convoy/1.0"},
				"X-Custom":      {"custom-value"},
			},
			expected: map[string]string{
				"authorization": "***",
				"content-type":  "application/json",
				"user-agent":    "convoy/1.0",
				"x-custom":      "***",
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

func TestProjectIDForInstrumentationUsesContextProject(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "/api/v1/projects/url-project/events", nil)
	assert.NoError(t, err)

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("projectID", "url-project")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, convoy.ProjectCtx, &datastore.Project{UID: "auth-project"})

	assert.Equal(t, "auth-project", projectIDForInstrumentation(req.WithContext(ctx)))
}

func TestProjectIDForInstrumentationFallsBackToURLProject(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "/api/v1/projects/url-project/events", nil)
	assert.NoError(t, err)

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("projectID", "url-project")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)

	assert.Equal(t, "url-project", projectIDForInstrumentation(req.WithContext(ctx)))
}

func TestRedactSensitiveHeaders(t *testing.T) {
	t.Run("masks sensitive values and preserves safe ones without mutating input", func(t *testing.T) {
		input := map[string]string{
			"Authorization":      "Bearer live-secret-9999",
			"X-My-Api-Key":       "abc",
			"Content-Type":       "application/json",
			"X-Custom":           "cv",
			"X-Convoy-Signature": "t=1,v1=abc",
			"X-Hub-Signature":    "sha1=def",
		}

		result := RedactSensitiveHeaders(input)

		// Long credential: only the trailing characters are revealed for debugging.
		assert.Equal(t, "***9999", result["Authorization"])
		// Short credentials are fully masked (nothing safe to reveal).
		assert.Equal(t, "***", result["X-My-Api-Key"])
		assert.Equal(t, "application/json", result["Content-Type"])
		// Non-allowlisted custom header is masked (allowlist behaviour).
		assert.Equal(t, "***", result["X-Custom"])
		// Signatures are HMACs the receiver verifies (not credentials) and are
		// surfaced in the dashboard for debugging, so they stay visible.
		assert.Equal(t, "t=1,v1=abc", result["X-Convoy-Signature"])
		assert.Equal(t, "sha1=def", result["X-Hub-Signature"])

		// Input map must be untouched so the stored/dispatched values survive.
		assert.Equal(t, "Bearer live-secret-9999", input["Authorization"])
		assert.Equal(t, "abc", input["X-My-Api-Key"])
	})

	t.Run("nil input returns nil", func(t *testing.T) {
		assert.Nil(t, RedactSensitiveHeaders(nil))
	})
}

func TestRedactSensitiveMultiHeaders(t *testing.T) {
	t.Run("masks sensitive values and preserves safe ones without mutating input", func(t *testing.T) {
		input := map[string][]string{
			"Authorization":      {"Bearer live-secret-9999"},
			"Content-Type":       {"application/json"},
			"X-Custom":           {"aaaaaaaa1234", "bb"},
			"X-Convoy-Signature": {"t=1,v1=abc"},
		}

		result := RedactSensitiveMultiHeaders(input)

		assert.Equal(t, []string{"***9999"}, result["Authorization"])
		assert.Equal(t, []string{"application/json"}, result["Content-Type"])
		// Each value masked independently: long reveals last 4, short fully masks.
		assert.Equal(t, []string{"***1234", "***"}, result["X-Custom"])
		// Signature stays visible for dashboard debugging.
		assert.Equal(t, []string{"t=1,v1=abc"}, result["X-Convoy-Signature"])

		assert.Equal(t, []string{"Bearer live-secret-9999"}, input["Authorization"])
	})

	t.Run("nil input returns nil", func(t *testing.T) {
		assert.Nil(t, RedactSensitiveMultiHeaders(nil))
	})
}

func TestSensitiveHeaderValuesNeverLogged(t *testing.T) {
	rawValue := "super-secret-value-that-must-not-be-logged"

	headers := []string{
		// explicit sensitiveHeaders
		"Authorization",
		"Proxy-Authorization",
		"Cookie",
		"Set-Cookie",
		"X-Forwarded-For",
		"X-Real-Ip",
		"X-Api-Key",
		"X-Auth-Token",
		// sensitivePatterns
		"X-Hub-Signature",
		"X-Webhook-Secret",
		"X-Access-Token",
		"X-Encryption-Key",
		"X-User-Password",
		"X-Service-Credential",
	}

	for _, name := range headers {
		t.Run(name, func(t *testing.T) {
			h := http.Header{}
			h.Set(name, rawValue)

			result := headerFields(h)

			for _, v := range result {
				assert.NotEqual(t, rawValue, v,
					"sensitive header %q raw value must not appear in log output", name)
			}
		})
	}
}
