package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/oklog/ulid/v2"
)

type InviteUserService struct {
	Queue      queue.Queuer
	InviteRepo datastore.OrganisationInviteRepository

	InviteeEmail string
	Role         auth.Role
	User         *datastore.User
	Organisation *datastore.Organisation
	Licenser     license.Licenser
}

func (iu *InviteUserService) Run(ctx context.Context) (*datastore.OrganisationInvite, error) {
	ok, err := iu.Licenser.CreateUser(ctx)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	if !ok {
		return nil, &ServiceError{ErrMsg: ErrUserLimit.Error()}
	}

	iv := &datastore.OrganisationInvite{
		UID:            ulid.Make().String(),
		OrganisationID: iu.Organisation.UID,
		InviteeEmail:   iu.InviteeEmail,
		Token:          uniuri.NewLen(64),
		Role:           iu.Role,
		Status:         datastore.InviteStatusPending,
		ExpiresAt:      time.Now().Add(time.Hour * 24 * 14), // expires in 2 weeks.
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if !iu.Licenser.MultiPlayerMode() {
		iu.Role.Type = auth.RoleSuperUser
	}

	err = iu.InviteRepo.CreateOrganisationInvite(ctx, iv)
	if err != nil {
		errMsg := "failed to invite member"
		log.FromContext(ctx).WithError(err).Error(errMsg)

		if strings.Contains(err.Error(), "duplicate") && strings.Contains(err.Error(), "organisation_invites_invitee_email") {
			return nil, &ServiceError{ErrMsg: "an invite for this email already exists", Err: err}
		}

		return nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	err = sendInviteEmail(ctx, iv, iu.User, iu.Organisation, iu.Queue)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to send email invite")
	}

	return iv, nil
}

func sendInviteEmail(ctx context.Context, iv *datastore.OrganisationInvite, user *datastore.User, org *datastore.Organisation, queuer queue.Queuer) error {
	cfg, err := config.Get()
	if err != nil {
		return err
	}

	baseURL := cfg.Host
	em := email.Message{
		Email:        iv.InviteeEmail,
		Subject:      "Convoy Organization Invite",
		TemplateName: email.TemplateOrganisationInvite,
		Params: map[string]string{
			"invite_url":        fmt.Sprintf("%s/accept-invite?invite-token=%s", baseURL, iv.Token),
			"organisation_name": org.Name,
			"inviter_name":      fmt.Sprintf("%s %s", user.FirstName, user.LastName),
			"expires_at":        iv.ExpiresAt.Format(time.RFC1123),
		},
	}

	bytes, err := msgpack.EncodeMsgPack(em)
	if err != nil {
		return nil
	}

	job := &queue.Job{
		Payload: bytes,
		Delay:   0,
	}

	err = queuer.Write(convoy.EmailProcessor, convoy.DefaultQueue, job)
	if err != nil {
		return err
	}

	return nil
}
