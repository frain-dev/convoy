package services

import (
	"context"
	"log/slog"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
)

type FindUserByInviteTokenService struct {
	Queue      queue.Queuer
	InviteRepo datastore.OrganisationInviteRepository
	OrgRepo    datastore.OrganisationRepository
	UserRepo   datastore.UserRepository

	Token string
}

func (ri *FindUserByInviteTokenService) Run(ctx context.Context) (*datastore.User, *datastore.OrganisationInvite, error) {
	iv, err := ri.InviteRepo.FetchOrganisationInviteByToken(ctx, ri.Token)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch organisation member invite by token and email", "error", err)
		return nil, nil, &ServiceError{ErrMsg: "failed to fetch organisation member invite", Err: err}
	}

	org, err := ri.OrgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		errMsg := "failed to fetch organisation by id"
		slog.ErrorContext(ctx, errMsg, "error", err)
		return nil, nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	iv.OrganisationName = org.Name

	user, err := ri.UserRepo.FindUserByEmail(ctx, iv.InviteeEmail)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, iv, nil
		}

		errMsg := "failed to fetch invited user"
		slog.ErrorContext(ctx, errMsg, "error", err)
		return nil, nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	return user, iv, nil
}
