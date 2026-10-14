package listener

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/services"
)

func TestAfterUpdate_EmitsMetaEventPerDeliveryStatus(t *testing.T) {
	tests := []struct {
		status   datastore.EventDeliveryStatus
		wantType datastore.HookEventType
	}{
		{datastore.SuccessEventStatus, datastore.EventDeliverySuccess},
		{datastore.FailureEventStatus, datastore.EventDeliveryFailed},
		{datastore.RetryEventStatus, datastore.EventDeliveryRetrying},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			queue := mocks.NewMockQueuer(ctrl)
			projectRepo := mocks.NewMockProjectRepository(ctrl)
			metaEventRepo := mocks.NewMockMetaEventRepository(ctrl)
			attemptsRepo := mocks.NewMockDeliveryAttemptsRepository(ctrl)

			projectRepo.EXPECT().FetchProjectByID(gomock.Any(), "proj-1").Return(&datastore.Project{
				UID: "proj-1",
				Config: &datastore.ProjectConfig{
					Strategy: &datastore.StrategyConfiguration{},
					MetaEvent: &datastore.MetaEventConfiguration{
						IsEnabled: true,
						EventType: []string{string(tc.wantType)},
					},
				},
			}, nil)
			attemptsRepo.EXPECT().FindDeliveryAttempts(gomock.Any(), "ed-1").Return(nil, nil)
			metaEventRepo.EXPECT().CreateMetaEvent(gomock.Any(), gomock.Any()).DoAndReturn(
				func(_ context.Context, m *datastore.MetaEvent) error {
					require.Equal(t, string(tc.wantType), m.EventType)
					return nil
				})
			queue.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			lo := logger.New("test", slog.LevelError)
			l := &EventDeliveryListener{
				mEvent:       services.NewMetaEvent(queue, projectRepo, metaEventRepo, lo),
				attemptsRepo: attemptsRepo,
				logger:       lo,
			}
			l.AfterUpdate(context.Background(), &datastore.EventDelivery{UID: "ed-1", ProjectID: "proj-1", Status: tc.status}, nil)
		})
	}
}

func TestAfterUpdate_IgnoresNonNotifiableStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	lo := logger.New("test", slog.LevelError)
	l := &EventDeliveryListener{
		mEvent: services.NewMetaEvent(mocks.NewMockQueuer(ctrl), mocks.NewMockProjectRepository(ctrl),
			mocks.NewMockMetaEventRepository(ctrl), lo),
		attemptsRepo: mocks.NewMockDeliveryAttemptsRepository(ctrl),
		logger:       lo,
	}

	// Strict mocks fail the test on any repository, project or queue call.
	l.AfterUpdate(context.Background(), &datastore.EventDelivery{UID: "ed-1", ProjectID: "proj-1", Status: datastore.ProcessingEventStatus}, nil)
}

func TestGetMetaEventDelivery_CopiesAcknowledgedAt(t *testing.T) {
	ackedAt := time.Now().UTC()
	eventDelivery := &datastore.EventDelivery{
		UID:            "ed-1",
		ProjectID:      "proj-1",
		Status:         datastore.SuccessEventStatus,
		AcknowledgedAt: null.TimeFrom(ackedAt),
	}

	meta := getMetaEventDelivery(eventDelivery)

	require.True(t, meta.AcknowledgedAt.Valid)
	require.True(t, meta.AcknowledgedAt.Time.Equal(ackedAt))
}

func TestMetaEventDelivery_PayloadIncludesTimestamps(t *testing.T) {
	requestedAt := time.Now().UTC()
	respondedAt := requestedAt.Add(40 * time.Millisecond)

	meta := getMetaEventDelivery(&datastore.EventDelivery{
		UID:            "ed-2",
		ProjectID:      "proj-1",
		Status:         datastore.SuccessEventStatus,
		AcknowledgedAt: null.TimeFrom(requestedAt),
	})
	meta.DeliveryAttempt = datastore.DeliveryAttempt{
		UID:         "att-1",
		RequestedAt: null.TimeFrom(requestedAt),
		RespondedAt: null.TimeFrom(respondedAt),
	}

	raw, err := json.Marshal(meta)
	require.NoError(t, err)

	var payload struct {
		AcknowledgedAt *time.Time `json:"acknowledged_at"`
		Attempt        struct {
			RequestedAt *time.Time `json:"requested_at"`
			RespondedAt *time.Time `json:"responded_at"`
		} `json:"attempt"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))

	require.NotNil(t, payload.AcknowledgedAt)
	require.NotNil(t, payload.Attempt.RequestedAt)
	require.NotNil(t, payload.Attempt.RespondedAt)
}
