package task

import (
	"testing"

	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/stretchr/testify/require"
)

func TestPrepareOutboundHeaders_AllowsCustomWhenEntitled(t *testing.T) {
	headers := httpheader.HTTPHeader{
		"User-Agent": []string{"PartnerBot/1"},
		"X-Custom":   []string{"keep"},
	}

	out := prepareOutboundHeaders(headers, true)
	require.Equal(t, []string{"PartnerBot/1"}, out["User-Agent"])
	require.Equal(t, []string{"keep"}, out["X-Custom"])
}

func TestPrepareOutboundHeaders_StripsCustomWhenNotEntitled(t *testing.T) {
	headers := httpheader.HTTPHeader{
		"User-Agent": []string{"PartnerBot/1"},
		"user-agent": []string{"also-custom"},
		"X-Custom":   []string{"keep"},
	}

	out := prepareOutboundHeaders(headers, false)
	_, hasUA := out["User-Agent"]
	_, hasLower := out["user-agent"]
	require.False(t, hasUA)
	require.False(t, hasLower)
	require.Equal(t, []string{"keep"}, out["X-Custom"])
	// Original map must stay intact for stored delivery headers.
	require.Equal(t, []string{"PartnerBot/1"}, headers["User-Agent"])
}

func TestPrepareOutboundHeaders_NilPassthrough(t *testing.T) {
	require.Nil(t, prepareOutboundHeaders(nil, false))
}
