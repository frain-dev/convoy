package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type CancelOrgMemberService struct {
	Queue      queue.Queuer
	DB         database.Database
	InviteRepo datastore.OrganisationInviteRepository

	InviteID string
}

func (co *CancelOrgMemberService) Run(ctx context.Context) (*datastore.OrganisationInvite, error) {
	iv, err := co.InviteRepo.FetchOrganisationInviteByID(ctx, co.InviteID)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fetch organisation by invitee id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation by invitee id"))
	}

	if iv.Status == datastore.InviteStatusCancelled {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("organisation member invite is already cancelled"))
	}

	iv.Status = datastore.InviteStatusCancelled
	iv.DeletedAt = null.NewTime(time.Now(), true)

	err = co.InviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update organisation member invite")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation member invite"))
	}
	return iv, nil
}
