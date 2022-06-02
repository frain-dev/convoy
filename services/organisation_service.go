package services

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

type OrganisationService struct {
	orgRepo datastore.OrganisationRepository
}

func NewOrganisationService(orgRepo datastore.OrganisationRepository) *OrganisationService {
	return &OrganisationService{orgRepo: orgRepo}
}

func (os *OrganisationService) CreateOrganisation(ctx context.Context, newOrg *models.Organisation, user *datastore.User) (*datastore.Organisation, error) {
	err := util.Validate(newOrg)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	org := &datastore.Organisation{
		UID:            uuid.NewString(),
		OwnerID:        user.UID,
		Name:           newOrg.Name,
		DocumentStatus: datastore.ActiveDocumentStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
	}

	err = os.orgRepo.CreateOrganisation(ctx, org)
	if err != nil {
		log.WithError(err).Error("failed to create organisation")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create organisation"))
	}

	return org, nil
}

func (os *OrganisationService) UpdateOrganisation(ctx context.Context, org *datastore.Organisation, update *models.Organisation) (*datastore.Organisation, error) {
	err := util.Validate(update)
	if err != nil {
		log.WithError(err).Error("failed to validate organisation update")
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	org.Name = update.Name
	err = os.orgRepo.UpdateOrganisation(ctx, org)
	if err != nil {
		log.WithError(err).Error("failed to to update organisation")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to update organisation"))
	}

	return org, nil
}

func (os *OrganisationService) FindOrganisationByID(ctx context.Context, id string) (*datastore.Organisation, error) {
	org, err := os.orgRepo.FetchOrganisationByID(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to find organisation by id")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to find organisation by id"))
	}
	return org, err
}

func (os *OrganisationService) LoadOrganisationsPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.Organisation, datastore.PaginationData, error) {
	orgs, paginationData, err := os.orgRepo.LoadOrganisationsPaged(ctx, pageable)
	if err != nil {
		log.WithError(err).Error("failed to fetch organisations")
		return nil, datastore.PaginationData{}, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while fetching organisations"))
	}

	return orgs, paginationData, nil
}

func (os *OrganisationService) DeleteOrganisation(ctx context.Context, id string) error {
	err := os.orgRepo.DeleteOrganisation(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to delete organisation")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to delete organisation"))
	}
	return err
}
