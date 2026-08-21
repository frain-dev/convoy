package disable

import (
	"context"
	"reflect"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
)

func cbEnablementAbsent(cb OrgCBEnablement) bool {
	if cb == nil {
		return true
	}
	v := reflect.ValueOf(cb)
	return v.Kind() == reflect.Ptr && v.IsNil()
}

// OrgCBEnablement resolves per-org circuit-breaking enablement (override wins).
type OrgCBEnablement interface {
	EnabledForOrg(ctx context.Context, orgID string) bool
}

// CircuitBreakingEnabledForOrg reports whether circuit breaking is licensed and
// live-enabled for the org. It gates breaker admission (CanExecute short-circuit)
// and sampler enforcement, and ignores project.Config.DisableEndpoint so projects
// can run circuit breaking without auto-disable.
//
// Failure policy: a nil enablement resolver on a licensed instance reports
// not-enabled, which skips breaker admission and hands disable ownership to the
// retry-limit path when DisableEndpoint is on.
func CircuitBreakingEnabledForOrg(ctx context.Context, licenser license.Licenser, cbEnablement OrgCBEnablement, orgID string) bool {
	if !licenser.CircuitBreaking() || cbEnablementAbsent(cbEnablement) {
		return false
	}
	return cbEnablement.EnabledForOrg(ctx, orgID)
}

// Endpoint auto-disable ownership when project.Config.DisableEndpoint is on:
//
//   - Circuit breaker (sampler notification handler and per-delivery enforcement)
//     owns disable when CircuitBreakingEnabledForOrg is true for the org.
//   - Retry-limit exhaustion owns disable otherwise: unlicensed instances, or
//     licensed instances where the org has circuit breaking off via override.
//
// The paths are mutually exclusive; exactly one may disable for a given outage.
func RetryLimitOwnsEndpointDisable(ctx context.Context, licenser license.Licenser, cbEnablement OrgCBEnablement, project *datastore.Project) bool {
	if project == nil || project.Config == nil || !project.Config.DisableEndpoint {
		return false
	}
	if !licenser.CircuitBreaking() {
		return true
	}
	if cbEnablementAbsent(cbEnablement) {
		return true
	}
	return !cbEnablement.EnabledForOrg(ctx, project.OrganisationID)
}

func CircuitBreakerOwnsEndpointDisable(ctx context.Context, licenser license.Licenser, cbEnablement OrgCBEnablement, project *datastore.Project) bool {
	if project == nil || project.Config == nil || !project.Config.DisableEndpoint {
		return false
	}
	return CircuitBreakingEnabledForOrg(ctx, licenser, cbEnablement, project.OrganisationID)
}
