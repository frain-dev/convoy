package services

import (
	"context"
	"fmt"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type RegenerateProjectAPIKeyService struct {
	ProjectRepo datastore.ProjectRepository
	UserRepo    datastore.UserRepository
	APIKeyRepo  datastore.APIKeyRepository

	Project *datastore.Project
	Member  *datastore.OrganisationMember
}

func (ss *RegenerateProjectAPIKeyService) Run(ctx context.Context) (*datastore.APIKey, string, error) {
	// does the organisation member have access to this project they're trying to regenerate an api key for?
	if !ss.Member.Role.Type.Is(auth.RoleOrganisationAdmin) {
		return nil, "", &ServiceError{ErrMsg: "unauthorized to access project"}
	}

	apiKey, err := ss.APIKeyRepo.FindAPIKeyByProjectID(ctx, ss.Project.UID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch project api key")
		return nil, "", &ServiceError{ErrMsg: "failed to fetch api project key", Err: err}
	}

	err = ss.APIKeyRepo.RevokeAPIKeys(ctx, []string{apiKey.UID})
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to revoke api key")
		return nil, "", &ServiceError{ErrMsg: "failed to revoke api key", Err: err}
	}

	cak := CreateAPIKeyService{
		ProjectRepo: ss.ProjectRepo,
		APIKeyRepo:  ss.APIKeyRepo,
		Member:      ss.Member,
		NewApiKey: &models.APIKey{
			Name: fmt.Sprintf("%s's key", ss.Project.Name),
			Role: models.Role{
				Type:    auth.RoleAdmin,
				Project: ss.Project.UID,
			},
		},
	}

	apiKey, keyString, err := cak.Run(ctx)
	if err != nil {
		return nil, "", err
	}

	return apiKey, keyString, nil
}
