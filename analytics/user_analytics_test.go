package analytics

import (
	"errors"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideUserAnalytics(ctrl *gomock.Controller) *UserAnalytics {
	userRepo := mocks.NewMockUserRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newUserAnalytics(userRepo, client, TestInstanceID)
}

func Test_TrackUserAnalytics(t *testing.T) {

	tests := []struct {
		name    string
		dbFn    func(ua *UserAnalytics)
		wantErr bool
	}{
		{
			name: "should_track_user_analytics",
			dbFn: func(ua *UserAnalytics) {
				userRepo := ua.userRepo.(*mocks.MockUserRepository)
				userRepo.EXPECT().LoadUsersPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, nil)
			},
		},

		{
			name: "should_fail_to_track_user_analytics",
			dbFn: func(ua *UserAnalytics) {
				userRepo := ua.userRepo.(*mocks.MockUserRepository)
				userRepo.EXPECT().LoadUsersPaged(gomock.Any(), gomock.Any()).Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ua := provideUserAnalytics(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ua)
			}

			err := ua.Track()

			if tc.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}
