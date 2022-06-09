package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/notification"
	"github.com/frain-dev/convoy/notification/email"
	"net/http"
	"strings"
	"time"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganisationInviteService struct {
	em            notification.Sender
	orgRepo       datastore.OrganisationRepository
	userRepo      datastore.UserRepository
	orgMemberRepo datastore.OrganisationMemberRepository
	orgInviteRepo datastore.OrganisationInviteRepository
}

func NewOrganisationInviteService(orgRepo datastore.OrganisationRepository, userRepo datastore.UserRepository, orgMemberRepo datastore.OrganisationMemberRepository, orgInviteRepo datastore.OrganisationInviteRepository, em notification.Sender) *OrganisationInviteService {
	return &OrganisationInviteService{
		em:            em,
		orgRepo:       orgRepo,
		userRepo:      userRepo,
		orgMemberRepo: orgMemberRepo,
		orgInviteRepo: orgInviteRepo,
	}
}

func (ois *OrganisationInviteService) CreateOrganisationMemberInvite(ctx context.Context, newIV *models.OrganisationInvite, org *datastore.Organisation, user *datastore.User, baseURL string) (*datastore.OrganisationInvite, error) {
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
		ExpiresAt:      primitive.NewDateTimeFromTime(time.Now().Add(time.Hour * 24 * 14)), // expires in 2 weeks
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err = ois.orgInviteRepo.CreateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to create organisation member invite")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member invite"))
	}

	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	go ois.sendInviteEmail(context.Background(), iv, org, user, baseURL)

	return iv, nil
}

func (ois *OrganisationInviteService) sendInviteEmail(ctx context.Context, iv *datastore.OrganisationInvite, org *datastore.Organisation, user *datastore.User, baseURL string) {
	n := &notification.Notification{
		Email:             iv.InviteeEmail,
		EmailTemplateName: email.TemplateOrganisationInvite.String(),
		InviteURL:         fmt.Sprintf("%s/ui/organisations/process_invite?token=%s", baseURL, iv.Token),
		OrganisationName:  org.Name,
		InviterName:       fmt.Sprintf("%s %s", user.FirstName, user.LastName),
	}

	err := ois.em.SendNotification(ctx, n)
	if err != nil {
		log.WithError(err).Error("failed to send email notification")
	}
}

func (ois *OrganisationInviteService) ProcessOrganisationMemberInvite(ctx context.Context, token string, accepted bool, newUser *models.User) error {
	iv, err := ois.orgInviteRepo.FetchOrganisationInviteByToken(ctx, token)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation member invite by token and email")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation member invite"))
	}

	if iv.Status != datastore.InviteStatusPending {
		return NewServiceError(http.StatusBadRequest, fmt.Errorf("organisation member invite already %s", iv.Status.String()))
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	if now > iv.ExpiresAt {
		return NewServiceError(http.StatusBadRequest, errors.New("organisation member invite already expired"))
	}

	if !accepted {
		iv.Status = datastore.InviteStatusDeclined
		err = ois.orgInviteRepo.UpdateOrganisationInvite(ctx, iv)
		if err != nil {
			log.WithError(err).Error("failed to update declined organisation invite")
			return NewServiceError(http.StatusBadRequest, errors.New("failed to update declined organisation invite"))
		}
		return nil
	}

	user, err := ois.userRepo.FindUserByEmail(ctx, iv.InviteeEmail)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			user, err = ois.createNewUser(ctx, newUser, iv.InviteeEmail)
			if err != nil {
				return err
			}
		} else {
			log.WithError(err).Error("failed to find user by email")
			return NewServiceError(http.StatusBadRequest, errors.New("failed to find user by email"))
		}
	}

	org, err := ois.orgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation by id")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation by id"))
	}

	_, err = NewOrganisationMemberService(ois.orgMemberRepo).CreateOrganisationMember(ctx, org, user, &iv.Role)
	if err != nil {
		return err
	}

	iv.Status = datastore.InviteStatusAccepted
	err = ois.orgInviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to update accepted organisation invite")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to update accepted organisation invite"))
	}

	return nil
}

func (ois *OrganisationInviteService) createNewUser(ctx context.Context, newUser *models.User, email string) (*datastore.User, error) {
	if newUser == nil {
		return nil, NewServiceError(http.StatusBadRequest, errors.New("new user is nil"))
	}

	err := util.Validate(newUser)
	if err != nil {
		log.WithError(err).Error("failed to validate new user information")
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	p := datastore.Password{Plaintext: newUser.Password}
	err = p.GenerateHash()
	if err != nil {
		log.WithError(err).Error("failed to generate user password hash")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member invite"))
	}

	user := &datastore.User{
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

	err = ois.userRepo.CreateUser(ctx, user)
	if err != nil {
		log.WithError(err).Error("failed to create user")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create user"))
	}

	return user, nil
}
