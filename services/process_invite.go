package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
)

type ProcessInviteService struct {
	Queue         queue.Queuer
	InviteRepo    datastore.OrganisationInviteRepository
	UserRepo      datastore.UserRepository
	OrgRepo       datastore.OrganisationRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	Licenser      license.Licenser

	Token    string
	Accepted bool
	NewUser  *models.User // TODO: Login Invite with SSO
	Logger   log.Logger
}

var ErrUserLimit = errors.New("your instance has reached it's user limit, upgrade to add new users")
var ErrUserAlreadyExist = errors.New("user already exist")

func (pis *ProcessInviteService) Run(ctx context.Context) error {
	ok, err := pis.Licenser.CheckUserLimit(ctx)
	if err != nil {
		return &ServiceError{ErrMsg: err.Error()}
	}

	if !ok {
		return &ServiceError{ErrMsg: ErrUserLimit.Error()}
	}

	iv, err := pis.InviteRepo.FetchOrganisationInviteByToken(ctx, pis.Token)
	if err != nil {
		pis.Logger.ErrorContext(ctx, "failed to fetch organisation member invite by token and email", "error", err)
		return &ServiceError{ErrMsg: "failed to fetch organisation member invite", Err: err}
	}

	if iv.Status != datastore.InviteStatusPending {
		return &ServiceError{ErrMsg: fmt.Sprintf("organisation member invite already %s", iv.Status.String())}
	}

	if time.Now().After(iv.ExpiresAt) { // if current date has surpassed expiry date
		return &ServiceError{ErrMsg: "organisation member invite already expired"}
	}

	if !pis.Accepted {
		iv.Status = datastore.InviteStatusDeclined
		iv.DeletedAt = null.TimeFrom(time.Now())
		err = pis.InviteRepo.UpdateOrganisationInvite(ctx, iv)
		if err != nil {
			errMsg := "failed to update declined organisation invite"
			pis.Logger.ErrorContext(ctx, errMsg, "error", err)
			return &ServiceError{ErrMsg: errMsg, Err: err}
		}
		return nil
	}

	user, err := pis.UserRepo.FindUserByEmail(ctx, iv.InviteeEmail)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			user, err = pis.createNewUser(ctx, pis.NewUser, iv.InviteeEmail)
			if err != nil {
				return err
			}
		} else {
			errMsg := "failed to find user by email"
			pis.Logger.ErrorContext(ctx, errMsg, "error", err)
			return &ServiceError{ErrMsg: errMsg, Err: err}
		}
	}

	org, err := pis.OrgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		errMsg := "failed to fetch organisation by id"
		pis.Logger.ErrorContext(ctx, errMsg, "error", err)
		return &ServiceError{ErrMsg: errMsg, Err: err}
	}

	_, err = NewOrganisationMemberService(pis.OrgMemberRepo, pis.Licenser, pis.Logger).CreateOrganisationMember(ctx, org, user, &iv.Role)
	if err != nil {
		return err
	}

	iv.Status = datastore.InviteStatusAccepted
	iv.DeletedAt = null.TimeFrom(time.Now())
	err = pis.InviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		errMsg := "failed to update accepted organisation invite"
		pis.Logger.ErrorContext(ctx, errMsg, "error", err)
		return &ServiceError{ErrMsg: errMsg, Err: err}
	}

	return nil
}

func (pis *ProcessInviteService) createNewUser(ctx context.Context, newUser *models.User, email string) (*datastore.User, error) {
	if newUser == nil {
		return nil, &ServiceError{ErrMsg: "new user is nil", Err: nil}
	}

	err := util.Validate(newUser)
	if err != nil {
		pis.Logger.ErrorContext(ctx, "failed to validate new user information", "error", err)
		return nil, &ServiceError{ErrMsg: err.Error(), Err: nil}
	}

	p := datastore.Password{Plaintext: newUser.Password}
	err = p.GenerateHash()
	if err != nil {
		pis.Logger.ErrorContext(ctx, "failed to generate user password hash", "error", err)
		return nil, &ServiceError{ErrMsg: "failed to create organisation member invite", Err: err}
	}

	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: newUser.FirstName,
		LastName:  newUser.LastName,
		Email:     email,
		Password:  string(p.Hash),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = pis.UserRepo.CreateUser(ctx, user)
	if err != nil {
		errMsg := "failed to create user"
		pis.Logger.ErrorContext(ctx, errMsg, "error", err)
		return nil, &ServiceError{ErrMsg: errMsg, Err: err}
	}

	return user, nil
}
