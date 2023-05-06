package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
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
		return nil, nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation member invite"))
	}

	org, err := ri.OrgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch organisation by id")
		return nil, nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation by id"))
	}
	iv.OrganisationName = org.Name

	user, err := ri.UserRepo.FindUserByEmail(ctx, iv.InviteeEmail)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, iv, nil
		}

		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return user, iv, nil
}
