package keygen

import (
	"encoding/json"

	"github.com/frain-dev/convoy/datastore"
)

func communityLicenser(orgRepo datastore.OrganisationRepository, orgMemberRepo datastore.OrganisationMemberRepository) (*Licenser, error) {
	l := &Licenser{
		planType: CommunityPlan,
		featureList: map[Feature]Properties{
			CreateOrg:       {Limit: 1},
			CreateOrgMember: {Limit: 1},
		},
		orgRepo:       orgRepo,
		orgMemberRepo: orgMemberRepo,
	}

	featureListJSON, err := json.Marshal(l.featureList)
	if err != nil {
		return nil, err
	}

	l.featureListJSON = featureListJSON
	return l, nil
}
