package task

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/endpoints/disable"
	"github.com/frain-dev/convoy/internal/notifications"
	"github.com/frain-dev/convoy/internal/pkg/license"
)

// orgCBEnablement resolves per-org circuit-breaking enablement (override wins).
type orgCBEnablement = disable.OrgCBEnablement

func circuitBreakingEnabledForOrg(ctx context.Context, licenser license.Licenser, cbEnablement orgCBEnablement, orgID string) bool {
	return disable.CircuitBreakingEnabledForOrg(ctx, licenser, cbEnablement, orgID)
}

func retryLimitOwnsEndpointDisable(ctx context.Context, licenser license.Licenser, cbEnablement orgCBEnablement, project *datastore.Project) bool {
	return disable.RetryLimitOwnsEndpointDisable(ctx, licenser, cbEnablement, project)
}

func circuitBreakerOwnsEndpointDisable(ctx context.Context, licenser license.Licenser, cbEnablement orgCBEnablement, project *datastore.Project) bool {
	return disable.CircuitBreakerOwnsEndpointDisable(ctx, licenser, cbEnablement, project)
}

func notifyRetryLimitEndpointDisabled(
	ctx context.Context,
	statusChanged bool,
	deps EventDeliveryProcessorDeps,
	endpoint *datastore.Endpoint,
	project *datastore.Project,
	failureMsg, responseBody string,
	statusCode int,
) {
	if !statusChanged || !deps.Licenser.AdvancedEndpointMgmt() {
		return
	}

	notifications.SendEndpointNotification(ctx, endpoint, project, datastore.InactiveEndpointStatus, deps.Queue, true, failureMsg, responseBody, statusCode, deps.Logger)
}
