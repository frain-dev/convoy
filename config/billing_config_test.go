package config_test

import (
	"testing"

	"github.com/frain-dev/convoy/config"
)

func TestBillingConfiguration_Validate(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty URL", func(t *testing.T) {
		t.Parallel()
		var b config.BillingConfiguration
		if err := b.Validate(); err == nil {
			t.Fatal("expected error when URL empty")
		}
	})

	t.Run("accepts URL set", func(t *testing.T) {
		t.Parallel()
		b := config.BillingConfiguration{URL: "https://overwatch.example.com"}
		if err := b.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects invalid URL", func(t *testing.T) {
		t.Parallel()
		b := config.BillingConfiguration{URL: "not a url"}
		if err := b.Validate(); err == nil {
			t.Fatal("expected error when URL invalid")
		}
	})
}

func TestConfiguration_Mode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		apiKey   string
		licKey   string
		expected config.BillingMode
	}{
		{"cloud when API key set", "sk_x", "", config.BillingModeCloud},
		{"cloud when API key set even with license", "sk_x", "lk_x", config.BillingModeCloud},
		{"licensed when only license key set", "", "lk_x", config.BillingModeLicensed},
		{"unlicensed when neither set", "", "", config.BillingModeUnlicensed},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := config.Configuration{
				LicenseKey: tc.licKey,
				Billing:    config.BillingConfiguration{APIKey: tc.apiKey},
			}
			if got := c.Mode(); got != tc.expected {
				t.Fatalf("Mode = %q, want %q", got, tc.expected)
			}
			if c.IsCloud() != (tc.expected == config.BillingModeCloud) {
				t.Fatalf("IsCloud mismatch for mode %q", tc.expected)
			}
			if c.IsSelfHosted() != (tc.expected != config.BillingModeCloud) {
				t.Fatalf("IsSelfHosted mismatch for mode %q", tc.expected)
			}
		})
	}
}
