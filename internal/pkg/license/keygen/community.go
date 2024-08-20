package keygen

import (
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
)

func communityLicenser(orgRepo datastore.OrganisationRepository, orgMemberRepo datastore.OrganisationMemberRepository, projectRepo datastore.ProjectRepository) (*Licenser, error) {
	l := &Licenser{
		planType: CommunityPlan,
		featureList: map[Feature]Properties{
			CreateOrg:     {Limit: 1},
			CreateUser:    {Limit: 1},
			CreateProject: {Limit: 2},
		},
		orgRepo:       orgRepo,
		orgMemberRepo: orgMemberRepo,
		projectRepo:   projectRepo,
	}

	featureListJSON, err := json.Marshal(l.featureList)
	if err != nil {
		return nil, err
	}

	l.featureListJSON = featureListJSON
	return l, nil
}
