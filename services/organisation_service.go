package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganisationService struct {
	orgRepo       datastore.OrganisationRepository
	orgMemberRepo datastore.OrganisationMemberRepository
}

func NewOrganisationService(orgRepo datastore.OrganisationRepository, orgMemberRepo datastore.OrganisationMemberRepository) *OrganisationService {
	return &OrganisationService{orgRepo: orgRepo, orgMemberRepo: orgMemberRepo}
}

func (os *OrganisationService) CreateOrganisation(ctx context.Context, newOrg *models.Organisation, user *datastore.User) (*datastore.Organisation, error) {
	err := util.Validate(newOrg)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	org := &datastore.Organisation{
		UID:       uuid.NewString(),
		OwnerID:   user.UID,
		Name:      newOrg.Name,
		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}

	err = os.orgRepo.CreateOrganisation(ctx, org)
	if err != nil {
		log.WithError(err).Error("failed to create organisation")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation"))
	}

	_, err = NewOrganisationMemberService(os.orgMemberRepo).CreateOrganisationMember(ctx, org, user, &auth.Role{Type: auth.RoleSuperUser})
	if err != nil {
		log.WithError(err).Error("failed to create super_user member for organisation owner")
	}

	return org, nil
}

func (os *OrganisationService) UpdateOrganisation(ctx context.Context, org *datastore.Organisation, update *models.Organisation) (*datastore.Organisation, error) {
	err := util.Validate(update)
	if err != nil {
		log.WithError(err).Error("failed to validate organisation update")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	org.Name = update.Name
	err = os.orgRepo.UpdateOrganisation(ctx, org)
	if err != nil {
		log.WithError(err).Error("failed to to update organisation")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation"))
	}

	return org, nil
}

func (os *OrganisationService) FindOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	org, err := os.orgRepo.FetchOrganisationByID(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to find organisation by id")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to find organisation by id"))
	}
	return org, err
}

func (os *OrganisationService) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	orgs, paginationData, err := os.orgRepo.LoadOrganisationsPaged(ctx, pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisations")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while fetching organisations"))
	}

	return orgs, paginationData, nil
}

func (os *OrganisationService) LoadUserOrganisationsPaged(ctx context.Context, user *datastore.User, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	orgs, paginationData, err := os.orgMemberRepo.LoadUserOrganisationsPaged(ctx, user.UID, pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch user organisations")
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while fetching user organisations"))
	}

	return orgs, paginationData, nil
}

func (os *OrganisationService) DeleteOrganisation(ctx context.Context, id string) error {
	err := os.orgRepo.DeleteOrganisation(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to delete organisation")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete organisation"))
	}
	return err
}
