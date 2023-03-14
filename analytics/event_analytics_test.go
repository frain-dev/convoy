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
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newEventAnalytics(eventRepo, projectRepo, orgRepo, client, TestInstanceID)
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
				projectRepo := ea.projectRepo.(*mocks.MockProjectRepository)
				eventRepo := ea.eventRepo.(*mocks.MockEventRepository)
				orgRepo := ea.orgRepo.(*mocks.MockOrganisationRepository)
				gomock.InOrder(
					orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return([]datastore.Organisation{{UID: "123"}}, datastore.PaginationData{}, nil),
					projectRepo.EXPECT().LoadProjects(gomock.Any(), gomock.Any()).Return([]*datastore.Project{{UID: "123456", Name: "test-project"}}, nil),
					eventRepo.EXPECT().LoadEventsPaged(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil),
					orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil),
				)
			},
		},

		{
			name: "should_fail_to_track_event_analytics",
			dbFn: func(ea *EventAnalytics) {
				orgRepo := ea.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, errors.New("failed"))
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
