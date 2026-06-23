package url

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEndpointTemplate(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		allowTemplate bool
		hasTemplate   bool
		err           error
	}{
		{
			name:          "plain URL without templates",
			rawURL:        "https://example.com/orders/callback",
			allowTemplate: false,
			hasTemplate:   false,
		},
		{
			name:          "path template allowed",
			rawURL:        "https://example.com/orders/{dynamic_value}/callback",
			allowTemplate: true,
			hasTemplate:   true,
		},
		{
			name:          "query template allowed",
			rawURL:        "https://example.com/callbacks/payment?reference={dynamic_value}",
			allowTemplate: true,
			hasTemplate:   true,
		},
		{
			name:          "path template blocked when disabled",
			rawURL:        "https://example.com/orders/{dynamic_value}/callback",
			allowTemplate: false,
			err:           ErrURLTemplateNotEnabled,
		},
		{
			name:          "template in host rejected",
			rawURL:        "https://{tenant}.example.com/callback",
			allowTemplate: true,
			err:           ErrURLTemplateUnsupportedPart,
		},
		{
			name:          "malformed template rejected",
			rawURL:        "https://example.com/orders/{dynamic-value}/callback",
			allowTemplate: true,
			err:           ErrURLTemplateInvalidToken,
		},
		{
			name:          "encoded braces are literal",
			rawURL:        "https://example.com/orders/%7Bdynamic_value%7D/callback",
			allowTemplate: false,
			hasTemplate:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, hasTemplate, err := ValidateEndpointTemplate(tt.rawURL, tt.allowTemplate)
			if tt.err != nil {
				require.True(t, errors.Is(err, tt.err), "expected %v, got %v", tt.err, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.hasTemplate, hasTemplate)
		})
	}
}

func TestTemplateMatches(t *testing.T) {
	tests := []struct {
		name        string
		templateURL string
		concreteURL string
		match       bool
		err         error
	}{
		{
			name:        "path template matches concrete URL",
			templateURL: "https://example.com/orders/{dynamic_value}/callback",
			concreteURL: "https://example.com/orders/ORD-123/callback",
			match:       true,
		},
		{
			name:        "query template matches concrete URL",
			templateURL: "https://example.com/callbacks/payment?reference={dynamic_value}",
			concreteURL: "https://example.com/callbacks/payment?reference=ORD-123",
			match:       true,
		},
		{
			name:        "query template matches concrete URL with extra query params",
			templateURL: "https://example.com/callbacks/payment?reference={dynamic_value}",
			concreteURL: "https://example.com/callbacks/payment?source=mobile&reference=ORD-123&currency=USD",
			match:       true,
		},
		{
			name:        "path template matches concrete URL with extra query params",
			templateURL: "https://example.com/orders/{dynamic_value}/callback",
			concreteURL: "https://example.com/orders/ORD-123/callback?source=mobile&currency=USD",
			match:       true,
		},
		{
			name:        "query template without path matches slash path concrete URL",
			templateURL: "https://example.com?reference={dynamic_value}",
			concreteURL: "https://example.com/?reference=ORD-123",
			match:       true,
		},
		{
			name:        "path template matches concrete URL with trailing slash",
			templateURL: "https://example.com/orders/{dynamic_value}/callback",
			concreteURL: "https://example.com/orders/ORD-123/callback/",
			match:       true,
		},
		{
			name:        "query template requires templated query param",
			templateURL: "https://example.com/callbacks/payment?reference={dynamic_value}",
			concreteURL: "https://example.com/callbacks/payment?source=mobile&currency=USD",
			match:       false,
		},
		{
			name:        "different host does not match",
			templateURL: "https://example.com/orders/{dynamic_value}/callback",
			concreteURL: "https://api.example.com/orders/ORD-123/callback",
			match:       false,
		},
		{
			name:        "different path suffix does not match",
			templateURL: "https://example.com/orders/{dynamic_value}/callback",
			concreteURL: "https://example.com/orders/ORD-123/status",
			match:       false,
		},
		{
			name:        "template segment does not cross path separator",
			templateURL: "https://example.com/orders/{dynamic_value}/callback",
			concreteURL: "https://example.com/orders/ORD-123/extra/callback",
			match:       false,
		},
		{
			name:        "plain URL is not a template",
			templateURL: "https://example.com/orders/callback",
			concreteURL: "https://example.com/orders/callback",
			match:       false,
		},
		{
			name:        "malformed template returns error",
			templateURL: "https://example.com/orders/{dynamic-value}/callback",
			concreteURL: "https://example.com/orders/ORD-123/callback",
			err:         ErrURLTemplateInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := TemplateMatches(tt.templateURL, tt.concreteURL)
			if tt.err != nil {
				require.True(t, errors.Is(err, tt.err), "expected %v, got %v", tt.err, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.match, match)
		})
	}
}
