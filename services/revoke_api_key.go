package services

import (
	"context"
	"log/slog"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type RevokePersonalAPIKeyService struct {
	ProjectRepo datastore.ProjectRepository
	UserRepo    datastore.UserRepository
	APIKeyRepo  datastore.APIKeyRepository

	UID  string
	User *datastore.User
}

func (ss *RevokePersonalAPIKeyService) Run(ctx context.Context) error {
	if util.IsStringEmpty(ss.UID) {
		return &ServiceError{ErrMsg: "key id is empty"}
	}

	apiKey, err := ss.APIKeyRepo.GetAPIKeyByID(ctx, ss.UID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch api key", "error", err)
		return &ServiceError{ErrMsg: "failed to fetch api key", Err: err}
	}

	if apiKey.Type != datastore.PersonalKey || apiKey.UserID != ss.User.UID {
		return &ServiceError{ErrMsg: "unauthorized"}
	}

	err = ss.APIKeyRepo.RevokeAPIKeys(ctx, []string{ss.UID})
	if err != nil {
		slog.ErrorContext(ctx, "failed to revoke api key", "error", err)
		return &ServiceError{ErrMsg: "failed to revoke api key", Err: err}
	}

	return nil
}
