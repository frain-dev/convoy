package analytics

import (
	"errors"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideProjectAnalytics(ctrl *gomock.Controller) *ProjectAnalytics {
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	client := NewNoopAnalyticsClient()

	return newProjectAnalytics(projectRepo, client, TestInstanceID)
}

func Test_TrackProjectAnalytics(t *testing.T) {
	tests := []struct {
		name    string
		dbFn    func(ga *ProjectAnalytics)
		wantErr bool
	}{
		{
			name: "should_track_project_analytics",
			dbFn: func(ga *ProjectAnalytics) {
				projectRepo := ga.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().LoadProjects(gomock.Any(), gomock.Any()).Return(nil, nil)
			},
		},

		{
			name: "should_fail_to_track_project_analytics",
			dbFn: func(ga *ProjectAnalytics) {
				projectRepo := ga.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().LoadProjects(gomock.Any(), gomock.Any()).Return(nil, errors.New("failed"))
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ga := provideProjectAnalytics(ctrl)

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
