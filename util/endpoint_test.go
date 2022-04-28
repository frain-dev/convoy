package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanEndpoint(t *testing.T) {

	tt := []struct {
		url      string
		hasError bool
	}{
		{"localhost:8080", true},
		{"https://localhost:8080", true},
		{"https://google.com", false},
		{"http://google.com", false},
		{"https://localhost", true},
		{"https://LocaLhOsT", true},
		{"https://127.0.0.1", true},
		{"https://GOOGLE.COM", false},
	}

	for _, v := range tt {
		url, err := CleanEndpoint(v.url)
		if v.hasError {
			require.Error(t, err)
			continue
		}

		require.NoError(t, err)
		require.Equal(t, v.url, url)
	}
}
