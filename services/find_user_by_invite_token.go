package services

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
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
		log.FromContext(ctx).WithError(err).Error("failed to fetch organisation member invite by token and email")
		return nil, nil, &ServiceError{ErrMsg: "failed to fetch organisation member invite", Err: err}
	}

	org, err := ri.OrgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		errMsg := "failed to fetch organisation by id"
		log.FromContext(ctx).WithError(err).Error(errMsg)
		return nil, nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	iv.OrganisationName = org.Name

	user, err := ri.UserRepo.FindUserByEmail(ctx, iv.InviteeEmail)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, iv, nil
		}

		errMsg := "failed to fetch invited user"
		log.FromContext(ctx).WithError(err).Error(errMsg)
		return nil, nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	return user, iv, nil
}
