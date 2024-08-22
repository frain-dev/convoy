package keygen

import (
	"github.com/frain-dev/convoy/datastore"
)

func communityLicenser(orgRepo datastore.OrganisationRepository, userRepo datastore.UserRepository, projectRepo datastore.ProjectRepository) *Licenser {
	return &Licenser{
		planType: CommunityPlan,
		featureList: map[Feature]*Properties{
			CreateOrg:     {Limit: 1},
			CreateUser:    {Limit: 1},
			CreateProject: {Limit: 2},
		},
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		projectRepo: projectRepo,
	}
}
