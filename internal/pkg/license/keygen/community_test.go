package keygen

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"
	"go.uber.org/mock/gomock"
)

func Test_communityLicenser(t *testing.T) {
	featureList := map[Feature]Properties{
		CreateOrg:     {Limit: 1},
		CreateUser:    {Limit: 1},
		CreateProject: {Limit: 2},
	}

	ctrl := gomock.NewController(t)
	orgRepo := mocks.NewMockOrganisationRepository(ctrl)
	userRepository := mocks.NewMockUserRepository(ctrl)
	projectRepo := mocks.NewMockProjectRepository(ctrl)

	l, err := communityLicenser(orgRepo, userRepository, projectRepo)
	require.NoError(t, err)

	require.Equal(t, featureList, l.featureList)
	require.Equal(t, orgRepo, l.orgRepo)
	require.Equal(t, userRepository, l.userRepo)
	require.Equal(t, projectRepo, l.projectRepo)

	v, err := json.Marshal(featureList)
	require.NoError(t, err)

	require.Equal(t, v, l.featureListJSON)
}
