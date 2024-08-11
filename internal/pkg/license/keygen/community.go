package keygen

import "github.com/frain-dev/convoy/datastore"

func communityLicenser(orgRepo datastore.OrganisationRepository, orgMemberRepo datastore.OrganisationMemberRepository) *KeygenLicenser {
	return &KeygenLicenser{
		planType: CommunityPlan,
		featureList: map[Feature]Properties{
			CreateOrg:       {Limit: 1},
			CreateOrgMember: {Limit: 1},
		},
		orgRepo:       orgRepo,
		orgMemberRepo: orgMemberRepo,
	}
}
