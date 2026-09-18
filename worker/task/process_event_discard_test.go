package task

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestQueueMatchingPersistsDiscardReason(t *testing.T) {
	for _, mode := range []datastore.DeliveryMode{datastore.AtLeastOnceDeliveryMode, datastore.AtMostOnceDeliveryMode} {
		for _, tc := range []struct {
			name   string
			status datastore.EndpointStatus
			auth   *datastore.EndpointAuthentication
			reason string
		}{
			{name: "paused", status: datastore.PausedEndpointStatus, reason: "endpoint status is paused"},
			{name: "inactive", status: datastore.InactiveEndpointStatus, reason: "endpoint status is inactive"},
			{name: "auth unavailable", status: datastore.ActiveEndpointStatus, auth: &datastore.EndpointAuthentication{Type: datastore.APIKeyAuthentication}, reason: "endpoint authentication is unavailable"},
		} {
			t.Run(string(mode)+"/"+tc.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				endpoint := &datastore.Endpoint{UID: "endpoint", Status: tc.status, Authentication: tc.auth}
				endpointRepo := mocks.NewMockEndpointRepository(ctrl)
				endpointRepo.EXPECT().FindEndpointByID(gomock.Any(), "endpoint", "project").Return(endpoint, nil)
				deliveries := mocks.NewMockEventDeliveryRepository(ctrl)
				deliveries.EXPECT().CreateEventDeliveries(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*datastore.EventDelivery) error {
					require.Len(t, rows, 1)
					require.Equal(t, datastore.DiscardedEventStatus, rows[0].Status)
					require.Equal(t, tc.reason, rows[0].Description)
					require.Equal(t, mode, rows[0].DeliveryMode)
					require.Zero(t, rows[0].Metadata.NumTrials)
					return nil
				})
				cfg := datastore.DefaultProjectConfig
				require.NoError(t, writeEventDeliveriesToQueue(t.Context(), WriteEventDeliveriesToQueueOptions{
					Project:       &datastore.Project{UID: "project", Config: &cfg},
					Event:         &datastore.Event{UID: "event", Data: []byte(`{"synthetic":true}`)},
					Subscriptions: []datastore.Subscription{{UID: "subscription", EndpointID: "endpoint", Type: datastore.SubscriptionTypeAPI, DeliveryMode: mode}},
					EndpointRepo:  endpointRepo, EventDeliveryRepo: deliveries, EventQueue: mocks.NewMockQueuer(ctrl), Logger: log.New("test", log.LevelError),
				}))
			})
		}
	}
}
