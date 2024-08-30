package keygen

import (
	"context"

	"github.com/keygen-sh/keygen-go/v3"

	"github.com/frain-dev/convoy/datastore"
)

const (
	projectLimit = 2
	orgLimit     = 1
	userLimit    = 1
)

func communityLicenser(ctx context.Context, orgRepo datastore.OrganisationRepository, userRepo datastore.UserRepository, projectRepo datastore.ProjectRepository) (*Licenser, error) {
	m, err := enforceProjectLimit(ctx, projectRepo)
	if err != nil {
		return nil, err
	}

	return &Licenser{
		planType: CommunityPlan,
		featureList: map[Feature]*Properties{
			CreateOrg:     {Limit: orgLimit},
			CreateUser:    {Limit: userLimit},
			CreateProject: {Limit: projectLimit},
		},
		license:         &keygen.License{},
		enabledProjects: m,
		orgRepo:         orgRepo,
		userRepo:        userRepo,
		projectRepo:     projectRepo,
	}, nil
}

func enforceProjectLimit(ctx context.Context, projectRepo datastore.ProjectRepository) (map[string]bool, error) {
	projects, err := projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{})
	if err != nil {
		return nil, err
	}

	if len(projects) > projectLimit {
		// enabled projects are not within accepted count, allow only the last projects to be active
		projects = projects[len(projects)-projectLimit:]
	}

	m := map[string]bool{}
	for _, p := range projects {
		m[p.UID] = true
	}

	return m, nil
}
