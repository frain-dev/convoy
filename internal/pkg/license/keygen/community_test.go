package keygen

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/datastore"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"go.uber.org/mock/gomock"
)

func Test_communityLicenser(t *testing.T) {
	testCases := []struct {
		name                string
		featureList         map[Feature]*Properties
		expectedFeatureList map[Feature]*Properties
		dbFn                func(projectRepo datastore.ProjectRepository)
	}{
		{
			name: "should_disable_projects",
			featureList: map[Feature]*Properties{
				CreateOrg:     {Limit: 1},
				CreateUser:    {Limit: 1},
				CreateProject: {Limit: 2},
			},
			dbFn: func(projectRepo datastore.ProjectRepository) {
				pr, _ := projectRepo.(*mocks.MockProjectRepository)
				pr.EXPECT().FetchEnabledProjectIDs(gomock.Any()).Times(1).Return([]string{"01111111", "02222", "033333", "044444"}, nil)
				pr.EXPECT().DisableProjects(gomock.Any(), []string{"01111111", "02222"}).Times(1).Return(nil)
			},
			expectedFeatureList: map[Feature]*Properties{
				CreateOrg:     {Limit: 1},
				CreateUser:    {Limit: 1},
				CreateProject: {Limit: 2},
			},
		},
		{
			name: "should_not_disable_projects",
			featureList: map[Feature]*Properties{
				CreateOrg:     {Limit: 1},
				CreateUser:    {Limit: 1},
				CreateProject: {Limit: 2},
			},
			dbFn: func(projectRepo datastore.ProjectRepository) {
				pr, _ := projectRepo.(*mocks.MockProjectRepository)
				pr.EXPECT().FetchEnabledProjectIDs(gomock.Any()).Times(1).Return([]string{"01111111", "02222"}, nil)
			},
			expectedFeatureList: map[Feature]*Properties{
				CreateOrg:     {Limit: 1},
				CreateUser:    {Limit: 1},
				CreateProject: {Limit: 2},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			orgRepo := mocks.NewMockOrganisationRepository(ctrl)
			userRepository := mocks.NewMockUserRepository(ctrl)
			projectRepo := mocks.NewMockProjectRepository(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(projectRepo)
			}

			l, err := communityLicenser(context.Background(), orgRepo, userRepository, projectRepo)
			require.NoError(t, err)

			require.Equal(t, tc.expectedFeatureList, l.featureList)
			require.Equal(t, orgRepo, l.orgRepo)
			require.Equal(t, userRepository, l.userRepo)
			require.Equal(t, projectRepo, l.projectRepo)
		})
	}
}
