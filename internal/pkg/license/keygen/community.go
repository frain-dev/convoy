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

	// we don't want CreateUser to show up on UI, so UI doesn't display the ability to
	// invite or register users, which will be blocked on backend anyway
	fl := make(map[Feature]Properties, len(l.featureList))
	for f, p := range l.featureList {
		if f != CreateUser {
			fl[f] = p
		}
	}

	featureListJSON, err := json.Marshal(fl)
	if err != nil {
		return nil, err
	}

	l.featureListJSON = featureListJSON
	return l, nil
}
