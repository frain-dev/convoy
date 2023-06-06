package url

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConcatQueryParams(t *testing.T) {
	tests := []struct {
		name      string
		targetURL string
		query     string
		expected  string
	}{
		{
			name:      "No Query Parameters",
			targetURL: "https://example.com",
			query:     "",
			expected:  "https://example.com",
		},
		{
			name:      "Single Query Parameter",
			targetURL: "https://example.com",
			query:     "param1=value1",
			expected:  "https://example.com?param1=value1",
		},
		{
			name:      "Multiple Query Parameters",
			targetURL: "https://example.com?source=facebook",
			query:     "param1=value1&param2=value2",
			expected:  "https://example.com?param1=value1&param2=value2&source=facebook",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ConcatQueryParams(test.targetURL, test.query)
			require.Nil(t, err)

			require.Equal(t, test.expected, result)
		})
	}
}
