package analytics

import (
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var TestInstanceID = "3d9c49ce-8367-43ec-8884-568c8d43faec"

func provideEventAnalytics(ctrl *gomock.Controller) *EventAnalytics {
	eventRepo := mocks.NewMockEventRepository(ctrl)
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newEventAnalytics(eventRepo, groupRepo, orgRepo, client, TestInstanceID)
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
				groupRepo := ea.groupRepo.(*mocks.MockGroupRepository)
				eventRepo := ea.eventRepo.(*mocks.MockEventRepository)
				orgRepo := ea.orgRepo.(*mocks.MockOrganisationRepository)
				groupRepo.EXPECT().LoadGroups(gomock.Any(), gomock.Any()).Return([]*datastore.Group{{UID: "123456", Name: "test-group"}}, nil)
				eventRepo.EXPECT().LoadEventsPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil)
				orgRepo.EXPECT().FetchOrganisationByID(gomock.Any(), gomock.Any()).Return(&datastore.Organisation{UID: "123456", Name: "test-org"}, nil)
			},
		},

		{
			name: "should_fail_to_track_event_analytics",
			dbFn: func(ea *EventAnalytics) {
				groupRepo := ea.groupRepo.(*mocks.MockGroupRepository)
				groupRepo.EXPECT().LoadGroups(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed"))
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
