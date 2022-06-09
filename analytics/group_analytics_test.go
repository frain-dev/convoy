package analytics

import (
	"errors"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideGroupAnalytics(ctrl *gomock.Controller) *GroupAnalytics {
	groupRepo := mocks.NewMockGroupRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newGroupAnalytics(groupRepo, client, OSSAnalyticsSource)
}

func Test_TrackGroupAnalytics(t *testing.T) {

	tests := []struct {
		name    string
		dbFn    func(ga *GroupAnalytics)
		wantErr bool
	}{
		{
			name: "should_track_group_analytics",
			dbFn: func(ga *GroupAnalytics) {
				groupRepo := ga.groupRepo.(*mocks.MockGroupRepository)
				groupRepo.EXPECT().LoadGroups(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},

		{
			name: "should_fail_to_track_group_analytics",
			dbFn: func(ga *GroupAnalytics) {
				groupRepo := ga.groupRepo.(*mocks.MockGroupRepository)
				groupRepo.EXPECT().LoadGroups(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ga := provideGroupAnalytics(ctrl)

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
