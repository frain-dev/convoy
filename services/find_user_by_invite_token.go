package services

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type FindUserByInviteTokenService struct {
	Queue      queue.Queuer
	InviteRepo datastore.OrganisationInviteRepository
	OrgRepo    datastore.OrganisationRepository
	UserRepo   datastore.UserRepository

	Token  string
	Logger log.Logger
}

func (ri *FindUserByInviteTokenService) Run(ctx context.Context) (*datastore.User, *datastore.OrganisationInvite, error) {
	iv, err := ri.InviteRepo.FetchOrganisationInviteByToken(ctx, ri.Token)
	if err != nil {
		ri.Logger.ErrorContext(ctx, "failed to fetch organisation member invite by token and email", "error", err)
		return nil, nil, &ServiceError{ErrMsg: "failed to fetch organisation member invite", Err: err}
	}

	org, err := ri.OrgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		errMsg := "failed to fetch organisation by id"
		ri.Logger.ErrorContext(ctx, errMsg, "error", err)
		return nil, nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	iv.OrganisationName = org.Name

	user, err := ri.UserRepo.FindUserByEmail(ctx, iv.InviteeEmail)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, iv, nil
		}

		errMsg := "failed to fetch invited user"
		ri.Logger.ErrorContext(ctx, errMsg, "error", err)
		return nil, nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	return user, iv, nil
}
