package services

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type ResendOrgMemberService struct {
	Queue      queue.Queuer
	InviteRepo datastore.OrganisationInviteRepository

	InviteID     string
	User         *datastore.User
	Organisation *datastore.Organisation
}

func (rs *ResendOrgMemberService) Run(ctx context.Context) (*datastore.OrganisationInvite, error) {
	iv, err := rs.InviteRepo.FetchOrganisationInviteByID(ctx, rs.InviteID)
	if err != nil {
		errMsg := "failed to fetch organisation by invitee id"
		log.FromContext(ctx).WithError(err).Error(errMsg)
		return nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	iv.ExpiresAt = time.Now().Add(time.Hour * 24 * 14) // expires in 2 weeks

	err = rs.InviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		errMsg := "failed to update organisation member invite"
		log.FromContext(ctx).WithError(err).Error(errMsg)
		return nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	err = sendInviteEmail(context.Background(), iv, rs.User, rs.Organisation, rs.Queue)
	if err != nil {
		return nil, err
	}
	return iv, nil
}
