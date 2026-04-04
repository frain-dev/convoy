package services

import (
	"context"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type CancelOrgMemberService struct {
	Queue      queue.Queuer
	InviteRepo datastore.OrganisationInviteRepository
	InviteID   string
	Logger     log.Logger
}

func (co *CancelOrgMemberService) Run(ctx context.Context) (*datastore.OrganisationInvite, error) {
	iv, err := co.InviteRepo.FetchOrganisationInviteByID(ctx, co.InviteID)
	if err != nil {
		errMsg := "failed to fetch organisation invite by id"
		co.Logger.ErrorContext(ctx, errMsg, "error", err)
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
		co.Logger.ErrorContext(ctx, errMsg, "error", err)
		return nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	return iv, nil
}
