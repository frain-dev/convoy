package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
}

func (iu *InviteUserService) Run(ctx context.Context) (*datastore.OrganisationInvite, error) {
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

	err := iu.InviteRepo.CreateOrganisationInvite(ctx, iv)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to invite member")
		return nil, err
	}

	err = iu.sendInviteEmail(ctx, iv)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to send email invite")
	}

	return iv, nil
}

func (iu *InviteUserService) sendInviteEmail(ctx context.Context, iv *datastore.OrganisationInvite) error {

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
			"organisation_name": iu.Organisation.Name,
			"inviter_name":      fmt.Sprintf("%s %s", iu.User.FirstName, iu.User.LastName),
			"expires_at":        iv.ExpiresAt.String(),
		},
	}

	buf, err := json.Marshal(em)
	if err != nil {
		return nil
	}

	job := &queue.Job{
		Payload: json.RawMessage(buf),
		Delay:   0,
	}

	err = iu.Queue.Write(convoy.EmailProcessor, convoy.DefaultQueue, job)
	if err != nil {
		return err
	}

	return nil
}
