package billing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanonicalOrigin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "lowercases host", raw: "https://Customer.example.com/", want: "https://customer.example.com"},
		{name: "drops default https port", raw: "https://customer.example.com:443", want: "https://customer.example.com"},
		{name: "keeps non-default port", raw: "http://localhost:4200", want: "http://localhost:4200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalOrigin(tt.raw)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCanonicalOriginRejectsAmbiguousHosts(t *testing.T) {
	tests := []string{
		"https://customer.example.com/path",
		"https://user@customer.example.com",
		"ftp://customer.example.com",
		"https://customer.example.com?x=1",
		"https://custómer.example.com",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			_, err := CanonicalOrigin(raw)
			require.Error(t, err)
		})
	}
}
