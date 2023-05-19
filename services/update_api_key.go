package services

import (
	"context"

	"github.com/frain-dev/convoy/auth"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

type UpdateAPIKeyService struct {
	ProjectRepo datastore.ProjectRepository
	UserRepo    datastore.UserRepository
	APIKeyRepo  datastore.APIKeyRepository

	UID  string
	Role *auth.Role
}

func (ss *UpdateAPIKeyService) Run(ctx context.Context) (*datastore.APIKey, error) {
	if util.IsStringEmpty(ss.UID) {
		return nil, &ServiceError{ErrMsg: "key id is empty"}
	}

	err := ss.Role.Validate("api key")
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("invalid api key role")
		return nil, &ServiceError{ErrMsg: "invalid api key role", Err: err}
	}

	_, err = ss.ProjectRepo.FetchProjectByID(ctx, ss.Role.Project)
	if err != nil {
		return nil, &ServiceError{ErrMsg: "invalid project", Err: err}
	}

	apiKey, err := ss.APIKeyRepo.FindAPIKeyByID(ctx, ss.UID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch api key")
		return nil, &ServiceError{ErrMsg: "failed to fetch api key", Err: err}
	}

	apiKey.Role = *ss.Role
	err = ss.APIKeyRepo.UpdateAPIKey(ctx, apiKey)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update api key")
		return nil, &ServiceError{ErrMsg: "failed to update api key", Err: err}
	}

	return apiKey, nil
}
