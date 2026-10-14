package dedup

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChecksumForValue_MatchesIngestChecksum(t *testing.T) {
	const source = "provider-webhooks"
	const value = "00000000-1111-4222-8333-444444444444"

	tests := []struct {
		name     string
		location string
		request  func() *http.Request
	}{
		{
			name:     "header",
			location: "request.header.X-GitHub-Delivery",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/ingest/abc", strings.NewReader(`{}`))
				r.Header.Set("Content-Type", "application/json")
				r.Header.Set("X-GitHub-Delivery", value)
				return r
			},
		},
		{
			name:     "nested json body",
			location: "request.body.data.requestId",
			request: func() *http.Request {
				r := httptest.NewRequest(http.MethodPost, "/ingest/abc", strings.NewReader(`{"data":{"requestId":"`+value+`"}}`))
				r.Header.Set("Content-Type", "application/json")
				return r
			},
		},
		{
			name:     "query param",
			location: "request.query.id",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, "/ingest/abc?id="+value, http.NoBody)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stored, err := NewDeDuper(context.Background(), tc.request(), nil).GenerateChecksum(source, []string{tc.location})
			require.NoError(t, err)
			require.Equal(t, stored, ChecksumForValue(source, value))
		})
	}
}

func TestChecksumForValue_DependsOnSourceName(t *testing.T) {
	require.NotEqual(t, ChecksumForValue("a", "v"), ChecksumForValue("b", "v"))
}
