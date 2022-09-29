package analytics

import (
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideActiveGroupAnalytics(ctrl *gomock.Controller) *ActiveGroupAnalytics {
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	eventRepo := mocks.NewMockEventRepository(ctrl)
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newActiveGroupAnalytics(groupRepo, eventRepo, orgRepo, client, TestInstanceID)
}

func Test_TrackActiveGroupAnalytics(t *testing.T) {
	tests := []struct {
		name    string
		dbFn    func(ga *ActiveGroupAnalytics)
		wantErr bool
	}{
		{
			name: "should_track_active_group_analytics",
			dbFn: func(ga *ActiveGroupAnalytics) {
				groupRepo := ga.groupRepo.(*mocks.MockGroupRepository)
				eventRepo := ga.eventRepo.(*mocks.MockEventRepository)
				orgRepo := ga.orgRepo.(*mocks.MockOrganisationRepository)
				gomock.InOrder(
					orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return([]datastore.Organisation{{UID: "123"}}, datastore.PaginationData{}, nil),
					groupRepo.EXPECT().LoadGroups(gomock.Any(), gomock.Any()).Return([]*datastore.Group{{UID: "123456", Name: "test"}}, nil),
					eventRepo.EXPECT().LoadEventsPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil),
					orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil),
				)
			},
		},

		{
			name: "should_fail_to_track_active_group_analytics",
			dbFn: func(ga *ActiveGroupAnalytics) {
				orgRepo := ga.orgRepo.(*mocks.MockOrganisationRepository)
				orgRepo.EXPECT().LoadOrganisationsPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ga := provideActiveGroupAnalytics(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ga)
			}

			err := ga.Track()

			if tc.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}
