package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
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
		log.FromContext(ctx).WithError(err).Error("failed to fetch organisation by invitee id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation by invitee id"))
	}
	iv.ExpiresAt = time.Now().Add(time.Hour * 24 * 14) // expires in 2 weeks

	err = rs.InviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to update organisation member invite")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation member invite"))
	}

	err = sendInviteEmail(context.Background(), iv, rs.User, rs.Organisation, rs.Queue)
	if err != nil {
		return nil, err
	}
	return iv, nil
}
