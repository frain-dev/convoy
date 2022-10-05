package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Advanced_Signatures(t *testing.T) {
	tests := map[string]struct {
		signature *Signature
		assertion require.ValueAssertionFunc
	}{
		"should_generate_multiple_signatures_for_rolled_secrets": {
			signature: &Signature{
				Payload: json.RawMessage(`{"b": {}, "e": "123", "a": 1}`),
				Schemes: []Scheme{
					{
						Secret:   []string{"older-expired-secret", "expired-secret", "new-secret"},
						Hash:     "SHA256",
						Encoding: "hex",
					},
				},
				Advanced: true,
			},
			assertion: assertSignatureVersionMapping([]string{
				"v0=a32dd2ae40bc678d67473679532190d7130f602c16b13c2ed868075b27789db9",
				"v0=e88d7e30fa711ac90e4c38710764e37e7dedd274d32bdef49f6627ea6d63d5f1",
				"v0=e438ab8eeeeb423cac6c5fa664d70116e72ec40c2416e912711cf4911fe06515",
			}),
		},
		"should_generate_multiple_signatures_for_multiple_schemes": {
			signature: &Signature{
				Payload: json.RawMessage(`{"b": {}, "e": "123", "a": 1}`),
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
				Advanced: true,
			},
			assertion: assertSignatureVersionMapping([]string{
				"v1=vh3shYSLzp2RdCAG4c+gtzkehvKA9yoC7VHS1M6GmpUSMaFRRr9UV+Vy7hdTtlbjMFA5ghC1Rl6+J0wLCwu3Cg==",
				"v0=6caf1bbbda9764281d0160dfa9a401c15186c8c3730e00e0276d894bace2f441",
			}),
		},
		"should_map_signatures_to_schemes_correctly": {
			signature: &Signature{
				Payload: json.RawMessage(`{"b": {}, "e": "123", "a": 1}`),
				Schemes: []Scheme{
					{
						Secret:   []string{"secret"},
						Hash:     "SHA256",
						Encoding: "hex",
					},
					{
						Secret:   []string{"secret"},
						Hash:     "SHA512",
						Encoding: "base64",
					},
				},
				Advanced: true,
			},
			assertion: assertSignatureVersionMapping([]string{
				"v0=ecf51ec27c7fe4c71298067890c6512dbb74186edf4eb5ee43c1c9c7d5c2b0e8",
				"v1=xdz+2j9aMVQUUjSy0KUz/CsjD4jaD6wHJGGf1c3eZzrWxHTf1cAjZ3aL07O9NZXMhg5gajfi+TYuBU1aoU18xA==",
			}),
		},
		"should_include_timestamp_in_computed_value": {
			signature: &Signature{
				Payload: json.RawMessage(`{"b": {}, "e": "123", "a": 1}`),
				Schemes: []Scheme{
					{
						Secret:   []string{"string"},
						Hash:     "SHA512",
						Encoding: "hex",
					},
				},
				Advanced: true,
			},
			assertion: assertSignatureIncludesTimestamp,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange

			// Act.
			sig, err := tc.signature.ComputeHeaderValue()
			require.NoError(t, err)

			// Assert.
			tc.assertion(t, sig)
		})
	}
}

func Test_Simple_Signatures(t *testing.T) {
	tests := map[string]struct {
		signature *Signature
		expected  string
		assertion require.ComparisonAssertionFunc
	}{
		"should_generate_simple_hex_signature": {
			signature: &Signature{
				Payload: json.RawMessage(`{"b": {}, "e": "123", "a": 1}`),
				Schemes: []Scheme{
					{
						Secret:   []string{"secret"},
						Hash:     "SHA512",
						Encoding: "hex",
					},
				},
				Advanced: false,
			},
			assertion: require.Equal,
			expected: "c5dcfeda3f5a3154145234b2d0a533fc2b230f88da0fac0724619fd5" +
				"cdde673ad6c474dfd5c02367768bd3b3bd3595cc860e606a37e2f9362e054d5aa14d7cc4",
		},
		"should_generate_simple_base64_signature": {
			signature: &Signature{
				Payload: json.RawMessage(`{"b": {}, "e": "123", "a": 1}`),
				Schemes: []Scheme{
					{
						Secret:   []string{"secret"},
						Hash:     "SHA512",
						Encoding: "base64",
					},
				},
				Advanced: false,
			},
			assertion: require.Equal,
			expected:  "xdz+2j9aMVQUUjSy0KUz/CsjD4jaD6wHJGGf1c3eZzrWxHTf1cAjZ3aL07O9NZXMhg5gajfi+TYuBU1aoU18xA==",
		},
		"should_generate_simple_signature_with_last_item_from_multiple_schemes": {
			signature: &Signature{
				Payload: json.RawMessage(`{"b": {}, "e": "123", "a": 1}`),
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
				Advanced: false,
			},
			assertion: require.Equal,
			expected:  "xdz+2j9aMVQUUjSy0KUz/CsjD4jaD6wHJGGf1c3eZzrWxHTf1cAjZ3aL07O9NZXMhg5gajfi+TYuBU1aoU18xA==",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange

			// Act.
			sig, err := tc.signature.ComputeHeaderValue()
			require.NoError(t, err)

			// Assert.
			tc.assertion(t, tc.expected, sig)
		})
	}
}

func assertSignatureIncludesTimestamp(t require.TestingT, v interface{}, args ...interface{}) {
	val, ok := v.(string)
	require.True(t, ok)

	require.Contains(t, parseSignature(val), "t=")
}

func assertSignatureVersionMapping(c []string) func(require.TestingT, interface{}, ...interface{}) {
	return func(t require.TestingT, s interface{}, args ...interface{}) {
		val, ok := s.(string)
		require.True(t, ok)

		ss := strings.Split(val, ",")
		for _, v := range c {
			require.Contains(t, ss, v)
		}
	}
}

// returns [ "t=", "v0=", "v1=" ... ]
func parseSignature(s string) (out []string) {
	ss := strings.Split(s, ",")

	for _, v := range ss {
		i := strings.Index(v, "=")
		out = append(out, v[:i+1])
	}

	return
}
