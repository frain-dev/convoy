package keygen

import (
	"context"

	"github.com/keygen-sh/keygen-go/v3"

	"github.com/frain-dev/convoy/datastore"
)

const (
	projectLimit = 2
	orgLimit     = 2
	userLimit    = 2
)

func communityLicenser(ctx context.Context, orgRepo datastore.OrganisationRepository, userRepo datastore.UserRepository, projectRepo datastore.ProjectRepository) (*Licenser, error) {
	l := &Licenser{
		planType: CommunityPlan,
		featureList: map[Feature]*Properties{
			CreateOrg:     {Limit: orgLimit},
			CreateUser:    {Limit: userLimit},
			CreateProject: {Limit: projectLimit},
		},
		license:     &keygen.License{},
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		projectRepo: projectRepo,
	}

	return l, enforceProjectLimit(ctx, projectRepo)
}

func enforceProjectLimit(ctx context.Context, projectRepo datastore.ProjectRepository) error {
	projectIDs, err := projectRepo.FetchEnabledProjectIDs(ctx)
	if err != nil {
		return err
	}

	if len(projectIDs) < projectLimit {
		// enabled projects are within accepted count, do nothing
		return nil
	}

	projectIDs = projectIDs[:len(projectIDs)-projectLimit] // remove last 2 ids

	if len(projectIDs) == 0 {
		return nil
	}

	err = projectRepo.DisableProjects(ctx, projectIDs) // disable the remaining projects
	return err
}
