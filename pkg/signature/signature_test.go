package signature

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
				generateTimestampFn: func() string {
					return "1257894000"
				},
				Advanced: true,
			},
			assertion: assertSignatureVersionMapping([]string{
				"v1=5b255b0b867c5540679bad7dfa11be73ffd4ee248647646fb325ccbe19167439",
				"v1=9942b27103f8f5f5ce4cd3751524dbb7087a596487c784655a33f528cd4d7400",
				"v1=d5589c4f5e51d74e6116c8d5db2704d76cdf88521e2cdd749d2d5091e3d6183c",
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
				generateTimestampFn: func() string {
					return "1257894000"
				},
				Advanced: true,
			},
			assertion: assertSignatureVersionMapping([]string{
				"v2=HEfgcRgjEFAl0rS/Vig/WvanDWsBNWx7y6htFUcou5hKXj4tPKy/4K/v8HXuIl2MeiPT8bYZvYHTd5ORhvN93Q==",
				"v1=c97c302e4a991a7d4a72b60c3f9a0c3adb3611cd0acc632994a72ace04d6509c",
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
				generateTimestampFn: func() string {
					return "1257894000"
				},
				Advanced: true,
			},
			assertion: assertSignatureVersionMapping([]string{
				"v1=c97c302e4a991a7d4a72b60c3f9a0c3adb3611cd0acc632994a72ace04d6509c",
				"v2=fKtYNaWP+THgLxwIifnkzGLBeCf8iWdGmFtKr0DM93+KCU1vksP8DmT8EbTwlF5Q0F2FmjOXxSxcUBkyuMlmVQ==",
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

func Test_ComputeHeaderValue_Errors(t *testing.T) {
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

// returns [ "t=", "v1=", "v2=" ... ]
func parseSignature(s string) (out []string) {
	ss := strings.Split(s, ",")

	for _, v := range ss {
		i := strings.Index(v, "=")
		out = append(out, v[:i+1])
	}

	return
}
