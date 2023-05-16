package services

import (
	"context"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
)

type CancelOrgMemberService struct {
	Queue      queue.Queuer
	InviteRepo datastore.OrganisationInviteRepository
	InviteID   string
}

func (co *CancelOrgMemberService) Run(ctx context.Context) (*datastore.OrganisationInvite, error) {
	iv, err := co.InviteRepo.FetchOrganisationInviteByID(ctx, co.InviteID)
	if err != nil {
		errMsg := "failed to fetch organisation invite by id"
		log.FromContext(ctx).WithError(err).Error(errMsg)
		return nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	if iv.Status == datastore.InviteStatusCancelled {
		return nil, &ServiceError{ErrMsg: "organisation member invite is already cancelled", Err: nil}
	}

	iv.Status = datastore.InviteStatusCancelled
	iv.DeletedAt = null.NewTime(time.Now(), true)

	err = co.InviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		errMsg := "failed to update organisation member invite"
		log.FromContext(ctx).WithError(err).Error(errMsg)
		return nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	return iv, nil
}
