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
	appRepo           datastore.ApplicationRepository
	orgRepo           datastore.OrganisationRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
}

func (os *OrganisationService) CreateOrganisation(ctx context.Context, newOrg *models.Organisation) (*datastore.Organisation, error) {
	err := util.Validate(newOrg)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	org := &datastore.Organisation{
		UID:            uuid.NewString(),
		OwnerID:        "", // TODO(daniel): to be completed when the user auth is completed by @dotunj
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
		return nil, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating organisation"))
	}

	return org, nil
}

func (os *OrganisationService) DeleteOrganisation(ctx context.Context, id string) error {
	err := os.orgRepo.DeleteOrganisation(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to delete organisation")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to delete organisation"))
	}
	return err
}
