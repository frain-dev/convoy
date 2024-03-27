package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanEndpoint(t *testing.T) {
	tt := []struct {
		url           string
		hasError      bool
		enforceSecure bool
	}{
		{"localhost:8080", true, false},
		{"https://localhost:8080", true, false},
		{"https://google.com", false, false},
		{"http://google.com", false, false},
		{"http://google.com", true, true},
		{"https://localhost", true, false},
		{"https://LocaLhOsT", true, false},
		{"https://127.0.0.1", true, false},
		{"https://GOOGLE.COM", false, false},
	}

	for _, v := range tt {
		url, err := ValidateEndpoint(v.url, v.enforceSecure)
		if v.hasError {
			require.Error(t, err)
			continue
		}

		require.NoError(t, err)
		require.Equal(t, v.url, url)
	}
}
