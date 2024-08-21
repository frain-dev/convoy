package keygen

import (
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
)

func communityLicenser(orgRepo datastore.OrganisationRepository, userRepo datastore.UserRepository, projectRepo datastore.ProjectRepository) (*Licenser, error) {
	l := &Licenser{
		planType: CommunityPlan,
		featureList: map[Feature]Properties{
			CreateUser:    {Limit: 1},
			CreateProject: {Limit: 2},
		},
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		projectRepo: projectRepo,
	}

	featureListJSON, err := json.Marshal(l.featureList)
	if err != nil {
		return nil, err
	}

	l.featureListJSON = featureListJSON
	return l, nil
}
