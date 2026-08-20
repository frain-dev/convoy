package disable_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/endpoints/disable"
	"github.com/frain-dev/convoy/mocks"
)

type stubCBEnablement struct {
	enabled bool
}

func (s stubCBEnablement) EnabledForOrg(_ context.Context, _ string) bool {
	return s.enabled
}

func TestCircuitBreakerOwnsEndpointDisable_RequiresDisableEndpoint(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().CircuitBreaking().AnyTimes().Return(true)

	cb := stubCBEnablement{enabled: true}
	project := &datastore.Project{
		OrganisationID: "org-1",
		Config:         &datastore.ProjectConfig{DisableEndpoint: false},
	}

	require.True(t, disable.CircuitBreakingEnabledForOrg(ctx, licenser, cb, project.OrganisationID))
	require.False(t, disable.CircuitBreakerOwnsEndpointDisable(ctx, licenser, cb, project))
}
