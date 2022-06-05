package analytics

import (
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideEventAnalytics(ctrl *gomock.Controller) *EventAnalytics {
	eventRepo := mocks.NewMockEventRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newEventAnalytics(eventRepo, client)
}

func Test_TrackEventAnalytics(t *testing.T) {

	tests := []struct {
		name    string
		dbFn    func(ea *EventAnalytics)
		wantErr bool
	}{
		{
			name: "should_track_event_analytics",
			dbFn: func(ea *EventAnalytics) {
				eventRepo := ea.eventRepo.(*mocks.MockEventRepository)
				eventRepo.EXPECT().LoadEventsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil)
			},
		},

		{
			name: "should_fail_to_track_event_analytics",
			dbFn: func(ea *EventAnalytics) {
				eventRepo := ea.eventRepo.(*mocks.MockEventRepository)
				eventRepo.EXPECT().LoadEventsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ea := provideEventAnalytics(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ea)
			}

			err := ea.Track()

			if tc.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}
