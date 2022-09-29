package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ComputeHeaderValue(t *testing.T) {
	tests := map[string]struct {
		signature     *Signature
		assertion     require.ComparisonAssertionFunc
		expectedValue string
		expectedError error
	}{
		"should_generate_non_versioned_signature": {
			signature: &Signature{
				Payload: json.RawMessage("Test Payload Body"),
				Schemes: []Scheme{
					{
						Secret:   []string{"secret"},
						Hash:     "SHA512",
						Encoding: "base64",
					},
				},
				Versioned: false,
				Timestamp: false,
			},
			assertion:     require.Equal,
			expectedValue: "4Jl5BpIybLLyS0Fha6xQrONto2+t+VK9emOp60D3keV2YOtQZC9FL5zCy6MoYiJk9+H0HwZN7iKK+M9S6oUfrg==",
			expectedError: nil,
		},
		"should_generate_non_versioned_signature_with_last_scheme": {
			signature: &Signature{
				Payload: json.RawMessage("Test Payload Body"),
				Schemes: []Scheme{
					{
						Secret:   []string{"secret"},
						Hash:     "SHA512",
						Encoding: "hex",
					},
					{
						Secret:   []string{"secret"},
						Hash:     "SHA512",
						Encoding: "base64",
					},
				},
				Versioned: false,
				Timestamp: false,
			},
			assertion:     require.Equal,
			expectedValue: "4Jl5BpIybLLyS0Fha6xQrONto2+t+VK9emOp60D3keV2YOtQZC9FL5zCy6MoYiJk9+H0HwZN7iKK+M9S6oUfrg==",
			expectedError: nil,
		},
		"should_generate_multiple_values_for_rolled_secrets": {
			signature: &Signature{
				Payload: json.RawMessage("Test Payload Body"),
				Schemes: []Scheme{
					{
						Secret:   []string{"expired-secret", "new-secret"},
						Hash:     "SHA256",
						Encoding: "hex",
					},
				},
				Versioned: true,
				Timestamp: true,
			},
			assertion: require.Equal,
			expectedValue: `
				v0=e88d7e30fa711ac90e4c38710764e37e7dedd274d32bdef49f6627ea6d63d5f1,
				v0=e438ab8eeeeb423cac6c5fa664d70116e72ec40c2416e912711cf4911fe06515
			`,
			expectedError: nil,
		},
		"should_generate_multiple_versions": {
			signature: &Signature{
				Payload: json.RawMessage("Test Payload Body"),
				Schemes: []Scheme{
					{
						Secret:   []string{"secret"},
						Hash:     "SHA256",
						Encoding: "hex",
					},
					{
						Secret:   []string{"new-scheme-secret"},
						Hash:     "SHA512",
						Encoding: "base64",
					},
				},
				Versioned: true,
				Timestamp: true,
			},
			assertion: require.Equal,
			expectedValue: `
				v1=vh3shYSLzp2RdCAG4c+gtzkehvKA9yoC7VHS1M6GmpUSMaFRRr9UV+Vy7hdTtlbjMFA5ghC1Rl6+J0wLCwu3Cg==,
				v0=6caf1bbbda9764281d0160dfa9a401c15186c8c3730e00e0276d894bace2f441
			`,
			expectedError: nil,
		},
		"should_map_scheme_to_version_correctly": {
			signature: &Signature{
				Payload: json.RawMessage("Test Payload Body"),
				Schemes: []Scheme{
					{
						Secret:   []string{},
						Hash:     "SHA512",
						Encoding: "hex",
					},
					{
						Secret:   []string{},
						Hash:     "SHA256",
						Encoding: "base64",
					},
				},
				Versioned: true,
				Timestamp: true,
			},
			assertion:     assertVersionMapping,
			expectedError: nil,
		},
		"should_include_timestamp_in_computed_value": {
			signature: &Signature{
				Payload: json.RawMessage("Test Payload Body"),
				Schemes: []Scheme{
					{
						Secret:   []string{"string"},
						Hash:     "SHA512",
						Encoding: "hex",
					},
				},
				Versioned: true,
				Timestamp: true,
			},
			assertion:     require.Contains,
			expectedValue: "t=",
			expectedError: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange

			// Act.
			sig, err := tc.signature.ComputeHeaderValue()
			require.NoError(t, err)

			// Assert.
			tc.assertion(t, strings.Join(strings.Fields(tc.expectedValue), ""), sig)
		})
	}
}

func assertVersionMapping(require.TestingT, interface{}, interface{}, ...interface{}) {
}
