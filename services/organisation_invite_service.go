package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/queue"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganisationInviteService struct {
	queue         queue.Queuer
	orgRepo       datastore.OrganisationRepository
	userRepo      datastore.UserRepository
	orgMemberRepo datastore.OrganisationMemberRepository
	orgInviteRepo datastore.OrganisationInviteRepository
}

func NewOrganisationInviteService(orgRepo datastore.OrganisationRepository, userRepo datastore.UserRepository, orgMemberRepo datastore.OrganisationMemberRepository, orgInviteRepo datastore.OrganisationInviteRepository, queue queue.Queuer) *OrganisationInviteService {
	return &OrganisationInviteService{
		queue:         queue,
		orgRepo:       orgRepo,
		userRepo:      userRepo,
		orgMemberRepo: orgMemberRepo,
		orgInviteRepo: orgInviteRepo,
	}
}

func (ois *OrganisationInviteService) CreateOrganisationMemberInvite(ctx context.Context, newIV *models.OrganisationInvite, org *datastore.Organisation, user *datastore.User, baseURL string) (*datastore.OrganisationInvite, error) {
	err := util.Validate(newIV)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	err = newIV.Role.Validate("organisation member")
	if err != nil {
		log.WithError(err).Error("failed to validate organisation member invite role")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	iv := &datastore.OrganisationInvite{
		UID:            uuid.NewString(),
		OrganisationID: org.UID,
		InviteeEmail:   newIV.InviteeEmail,
		Token:          uniuri.NewLen(64),
		Role:           newIV.Role,
		Status:         datastore.InviteStatusPending,
		ExpiresAt:      primitive.NewDateTimeFromTime(time.Now().Add(time.Hour * 24 * 14)), // expires in 2 weeks
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err = ois.orgInviteRepo.CreateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to create organisation member invite")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member invite"))
	}

	err = ois.sendInviteEmail(context.Background(), iv, org, user, baseURL)
	if err != nil {
		return nil, err
	}

	return iv, nil
}

func (ois *OrganisationInviteService) LoadOrganisationInvitesPaged(ctx context.Context, org *datastore.Organisation, inviteStatus datastore.InviteStatus, pageable datastore.Pageable) ([]datastore.OrganisationInvite, datastore.PaginationData, error) {
	invites, paginationData, err := ois.orgInviteRepo.LoadOrganisationsInvitesPaged(ctx, org.UID, inviteStatus, pageable)
	if err != nil {
		log.WithError(err).Error("failed to load organisation invites")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, errors.New("failed to load organisation invites"))
	}

	return invites, paginationData, nil
}

func (ois *OrganisationInviteService) sendInviteEmail(ctx context.Context, iv *datastore.OrganisationInvite, org *datastore.Organisation, user *datastore.User, baseURL string) error {
	em := email.Message{
		Email:        iv.InviteeEmail,
		Subject:      "Convoy Organization Invite",
		TemplateName: email.TemplateOrganisationInvite,
		Params: map[string]string{
			"invite_url":        fmt.Sprintf("%s/accept-invite?token=%s", baseURL, iv.Token),
			"organisation_name": org.Name,
			"inviter_name":      fmt.Sprintf("%s %s", user.FirstName, user.LastName),
			"expires_at":        iv.ExpiresAt.Time().String(),
		},
	}

	buf, err := json.Marshal(em)
	if err != nil {
		log.WithError(err).Error("failed to marshal notification payload")
		return nil
	}

	job := &queue.Job{
		Payload: json.RawMessage(buf),
		Delay:   0,
	}

	err = ois.queue.Write(convoy.EmailProcessor, convoy.DefaultQueue, job)
	if err != nil {
		log.WithError(err).Error("failed to write new notification to the queue")
	}

	return nil
}

func (ois *OrganisationInviteService) ProcessOrganisationMemberInvite(ctx context.Context, token string, accepted bool, newUser *models.User) error {
	iv, err := ois.orgInviteRepo.FetchOrganisationInviteByToken(ctx, token)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation member invite by token and email")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation member invite"))
	}

	if iv.Status != datastore.InviteStatusPending {
		return util.NewServiceError(http.StatusBadRequest, fmt.Errorf("organisation member invite already %s", iv.Status.String()))
	}

	now := primitive.NewDateTimeFromTime(time.Now())
	if now > iv.ExpiresAt {
		return util.NewServiceError(http.StatusBadRequest, errors.New("organisation member invite already expired"))
	}

	if !accepted {
		iv.Status = datastore.InviteStatusDeclined
		err = ois.orgInviteRepo.UpdateOrganisationInvite(ctx, iv)
		if err != nil {
			log.WithError(err).Error("failed to update declined organisation invite")
			return util.NewServiceError(http.StatusBadRequest, errors.New("failed to update declined organisation invite"))
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
			return util.NewServiceError(http.StatusBadRequest, errors.New("failed to find user by email"))
		}
	}

	org, err := ois.orgRepo.FetchOrganisationByID(ctx, iv.OrganisationID)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation by id")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation by id"))
	}

	_, err = NewOrganisationMemberService(ois.orgMemberRepo).CreateOrganisationMember(ctx, org, user, &iv.Role)
	if err != nil {
		return err
	}

	iv.Status = datastore.InviteStatusAccepted
	err = ois.orgInviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to update accepted organisation invite")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to update accepted organisation invite"))
	}

	return nil
}

func (ois *OrganisationInviteService) createNewUser(ctx context.Context, newUser *models.User, email string) (*datastore.User, error) {
	if newUser == nil {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("new user is nil"))
	}

	err := util.Validate(newUser)
	if err != nil {
		log.WithError(err).Error("failed to validate new user information")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	p := datastore.Password{Plaintext: newUser.Password}
	err = p.GenerateHash()
	if err != nil {
		log.WithError(err).Error("failed to generate user password hash")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation member invite"))
	}

	user := &datastore.User{
		UID:       uuid.NewString(),
		FirstName: newUser.FirstName,
		LastName:  newUser.LastName,
		Email:     email,
		Password:  string(p.Hash),
		// Role:          newUser.Role, // TODO(all): this role field shouldn't be in user.
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err = ois.userRepo.CreateUser(ctx, user)
	if err != nil {
		log.WithError(err).Error("failed to create user")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create user"))
	}

	return user, nil
}

func (ois *OrganisationInviteService) FindUserByInviteToken(ctx context.Context, token string) (*datastore.User, *datastore.OrganisationInvite, error) {
	iv, err := ois.orgInviteRepo.FetchOrganisationInviteByToken(ctx, token)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation member invite by token and email")
		return nil, nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation member invite"))
	}

	user, err := ois.userRepo.FindUserByEmail(ctx, iv.InviteeEmail)
	if err != nil {
		if err == datastore.ErrUserNotFound {
			return nil, iv, nil
		}

		return nil, nil, util.NewServiceError(http.StatusInternalServerError, err)
	}

	return user, iv, nil
}

func (ois *OrganisationInviteService) ResendOrganisationMemberInvite(ctx context.Context, inviteID string, org *datastore.Organisation, user *datastore.User, baseURL string) (*datastore.OrganisationInvite, error) {
	iv, err := ois.orgInviteRepo.FetchOrganisationInviteByID(ctx, inviteID)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation by invitee id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation by invitee id"))
	}
	iv.ExpiresAt = primitive.NewDateTimeFromTime(time.Now().Add(time.Hour * 24 * 14)) // expires in 2 weeks

	err = ois.orgInviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to update organisation member invite")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation member invite"))
	}

	err = ois.sendInviteEmail(context.Background(), iv, org, user, baseURL)
	if err != nil {
		return nil, err
	}
	return iv, nil
}

func (ois *OrganisationInviteService) CancelOrganisationMemberInvite(ctx context.Context, inviteID string) (*datastore.OrganisationInvite, error) {
	iv, err := ois.orgInviteRepo.FetchOrganisationInviteByID(ctx, inviteID)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisation by invitee id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to fetch organisation by invitee id"))
	}

	if iv.Status == datastore.InviteStatusCancelled {
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("organisation member invite is already cancelled"))
	}

	iv.Status = datastore.InviteStatusCancelled
	iv.DeletedAt = util.NewDateTime()

	err = ois.orgInviteRepo.UpdateOrganisationInvite(ctx, iv)
	if err != nil {
		log.WithError(err).Error("failed to update organisation member invite")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation member invite"))
	}
	return iv, nil
}
