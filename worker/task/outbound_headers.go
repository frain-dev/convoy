package task

import (
	"net/http"
	"strings"

	"github.com/frain-dev/convoy/pkg/httpheader"
)

// prepareOutboundHeaders returns headers for webhook dispatch.
// Failure policy: fail closed to Convoy branding when custom_user_agent is off —
// strip any User-Agent so the dispatcher fills Convoy/<version>. Does not mutate
// the stored delivery headers.
//
// customUserAgentAllowed comes from the instance Licenser (same pattern as
// MutualTLS / EndpointURLTemplates). Cloud UI may also surface org license_data
// flags; wire enforcement stays on the instance entitlement.
func prepareOutboundHeaders(headers httpheader.HTTPHeader, customUserAgentAllowed bool) httpheader.HTTPHeader {
	if customUserAgentAllowed || headers == nil {
		return headers
	}

	out := httpheader.HTTPHeader(http.Header(headers).Clone())
	for k := range out {
		if strings.EqualFold(k, "User-Agent") {
			delete(out, k)
		}
	}
	return out
}
