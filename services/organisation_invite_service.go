package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

type OrganisationInviteService struct {
	orgRepo       datastore.OrganisationRepository
	userRepo      datastore.UserRepository
	orgMemberRepo datastore.OrganisationMemberRepository
	orgInviteRepo datastore.OrganisationInviteRepository
}

func NewOrganisationInviteService(orgRepo datastore.OrganisationRepository, userRepo datastore.UserRepository, orgMemberRepo datastore.OrganisationMemberRepository, orgInviteRepo datastore.OrganisationInviteRepository) *OrganisationInviteService {
	return &OrganisationInviteService{orgRepo: orgRepo, userRepo: userRepo, orgMemberRepo: orgMemberRepo, orgInviteRepo: orgInviteRepo}
}

func (oi *OrganisationInviteService) CreateOrganisationMemberInvite(ctx context.Context, org *datastore.Organisation, newIV *models.OrganisationInvite) (*datastore.OrganisationInvite, error) {
	err := util.Validate(newIV)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	err = newIV.Role.Validate("organisation member")
	if err != nil {
		log.WithError(err).Error("failed to validate organisation member invite role")
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	iv := &datastore.OrganisationInvite{
		UID:            uuid.NewString(),
		OrganisationID: org.UID,
		InviteeEmail:   newIV.InviteeEmail,
		Token:          uniuri.NewLen(64),
		Role:           newIV.Role,
		Status:         datastore.InviteStatusPending,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	// TODO(daniel): send invite link to the invitee's email

	err = oi.orgInviteRepo.CreateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to create organisation member invite")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member invite"))
	}

	return iv, nil
}

func (oi *OrganisationInviteService) AcceptOrganisationMemberInvite(ctx context.Context, token string, email string, accepted bool, newUser *models.User) error {
	iv, err := oi.orgInviteRepo.FetchOrganisationInviteByTokenAndEmail(ctx, token, email)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation member invite by token and email")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member invite"))
	}

	if iv.Status != datastore.InviteStatusPending {
		return NewServiceError(http.StatusBadRequest, errors.New(fmt.Sprintf("organisation member invite already %s", iv.Status.String())))
	}

	org, err := oi.orgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		log.WithError(err).Error("failed to find organisation by id")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to find organisation by id"))
	}

	if accepted {
		var user *datastore.User

		if newUser != nil { // it is a new user
			err = newUser.Role.Validate("organisation member")
			if err != nil {
				log.WithError(err).Error("failed to validate organisation member invite role")
				return NewServiceError(http.StatusBadRequest, err)
			}

			p := datastore.Password{Plaintext: newUser.Password}
			err = p.GenerateHash()
			if err != nil {
				log.WithError(err).Error("failed to generate user password hash organisation member invite by token and email")
				return NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member invite"))
			}

			user = &datastore.User{
				UID:       uuid.NewString(),
				FirstName: newUser.FirstName,
				LastName:  newUser.LastName,
				Email:     email,
				Password:  string(p.Hash),
				//Role:          newUser.Role, // TODO(all): this role field shouldn't be in user.
				DocumentStatus: datastore.ActiveDocumentStatus,
				CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
				UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
			}

			err = oi.userRepo.CreateUser(ctx, user)
			if err != nil {
				log.WithError(err).Error("failed to create user")
				return NewServiceError(http.StatusBadRequest, errors.New("failed to create user"))
			}
		} else {
			user, err = oi.userRepo.FindUserByEmail(ctx, email)
			if err != nil {
				log.WithError(err).Error("failed to find user by email")
				return NewServiceError(http.StatusBadRequest, errors.New("failed to find user by email"))
			}
		}

		_, err = NewOrganisationMemberService(oi.orgMemberRepo).CreateOrganisationMember(ctx, org, user, &iv.Role)
		if err != nil {
			log.WithError(err).Error("failed to create organisation member")
			return NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member"))
		}

		iv.Status = datastore.InviteStatusAccepted
	} else {
		iv.Status = datastore.InviteStatusDeclined
	}

	err = oi.orgInviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to update organisation invite")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation invite"))
	}

	return nil
}
