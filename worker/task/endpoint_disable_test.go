package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type stubCBEnablement struct {
	enabled bool
}

func (s stubCBEnablement) EnabledForOrg(_ context.Context, _ string) bool {
	return s.enabled
}

func TestCircuitBreakingEnabledForOrg_IgnoresDisableEndpoint(t *testing.T) {
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

	require.True(t, circuitBreakingEnabledForOrg(ctx, licenser, cb, project.OrganisationID))
	require.False(t, circuitBreakerOwnsEndpointDisable(ctx, licenser, cb, project))
	require.False(t, retryLimitOwnsEndpointDisable(ctx, licenser, cb, project))
}

func TestEndpointDisableOwnership(t *testing.T) {
	ctx := context.Background()
	project := &datastore.Project{
		OrganisationID: "org-1",
		Config:         &datastore.ProjectConfig{DisableEndpoint: true},
	}

	tests := []struct {
		name            string
		licensedCB      bool
		orgOverrideOn   bool
		wantRetryOwns   bool
		wantBreakerOwns bool
	}{
		{
			name:            "licensed with org override on",
			licensedCB:      true,
			orgOverrideOn:   true,
			wantRetryOwns:   false,
			wantBreakerOwns: true,
		},
		{
			name:            "licensed with org override off",
			licensedCB:      true,
			orgOverrideOn:   false,
			wantRetryOwns:   true,
			wantBreakerOwns: false,
		},
		{
			name:            "unlicensed with org override on",
			licensedCB:      false,
			orgOverrideOn:   true,
			wantRetryOwns:   true,
			wantBreakerOwns: false,
		},
		{
			name:            "unlicensed with org override off",
			licensedCB:      false,
			orgOverrideOn:   false,
			wantRetryOwns:   true,
			wantBreakerOwns: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			licenser := mocks.NewMockLicenser(ctrl)
			licenser.EXPECT().CircuitBreaking().AnyTimes().Return(tc.licensedCB)

			cb := stubCBEnablement{enabled: tc.orgOverrideOn}

			require.Equal(t, tc.wantRetryOwns, retryLimitOwnsEndpointDisable(ctx, licenser, cb, project))
			require.Equal(t, tc.wantBreakerOwns, circuitBreakerOwnsEndpointDisable(ctx, licenser, cb, project))
			require.NotEqual(t, retryLimitOwnsEndpointDisable(ctx, licenser, cb, project), circuitBreakerOwnsEndpointDisable(ctx, licenser, cb, project))
		})
	}
}

func TestNotifyRetryLimitEndpointDisabled_RacingWriters(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)

	q := mocks.NewMockQueuer(ctrl)
	var notificationWrites int32
	q.EXPECT().
		Write(gomock.Any(), convoy.NotificationProcessor, convoy.DefaultQueue, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, _ *queue.Job) error {
			atomic.AddInt32(&notificationWrites, 1)
			return nil
		}).
		Times(1)

	deps := EventDeliveryProcessorDeps{
		Licenser: licenser,
		Queue:    q,
		Logger:   log.New("convoy", log.LevelError),
	}

	project := &datastore.Project{Name: "P1", LogoURL: "https://logo.example.com"}
	endpoint := &datastore.Endpoint{
		Name:         "E1",
		Url:          "https://e1.example.com",
		SupportEmail: "support@example.com",
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		notifyRetryLimitEndpointDisabled(ctx, true, deps, endpoint, project, "timeout", "", 0)
	}()
	go func() {
		defer wg.Done()
		notifyRetryLimitEndpointDisabled(ctx, false, deps, endpoint, project, "timeout", "", 0)
	}()
	wg.Wait()

	require.Equal(t, int32(1), atomic.LoadInt32(&notificationWrites))
}
