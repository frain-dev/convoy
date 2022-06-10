package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/auth"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GroupService struct {
	apiKeyRepo        datastore.APIKeyRepository
	appRepo           datastore.ApplicationRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	limiter           limiter.RateLimiter
}

func NewGroupService(apiKeyRepo datastore.APIKeyRepository, appRepo datastore.ApplicationRepository, groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, limiter limiter.RateLimiter) *GroupService {
	return &GroupService{
		apiKeyRepo:        apiKeyRepo,
		appRepo:           appRepo,
		groupRepo:         groupRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		limiter:           limiter,
	}
}

func (gs *GroupService) CreateGroup(ctx context.Context, newGroup *models.Group, org *datastore.Organisation) (*datastore.Group, *models.APIKeyResponse, error) {
	groupName := newGroup.Name

	// Apply Defaults
	c := &newGroup.Config
	if c.Signature == (datastore.SignatureConfiguration{}) {
		c.Signature = datastore.DefaultSignatureConfig
	}

	if c.Strategy == (datastore.StrategyConfiguration{}) {
		c.Strategy = datastore.DefaultStrategyConfig
	}

	if c.RateLimit == (datastore.RateLimitConfiguration{}) {
		c.RateLimit = datastore.DefaultRateLimitConfig
	}

	if newGroup.RateLimit == 0 {
		newGroup.RateLimit = convoy.RATE_LIMIT
	}

	if util.IsStringEmpty(newGroup.RateLimitDuration) {
		newGroup.RateLimitDuration = convoy.RATE_LIMIT_DURATION
	}

	err := util.Validate(newGroup)
	if err != nil {
		return nil, nil, NewServiceError(http.StatusBadRequest, err)
	}

	group := &datastore.Group{
		UID:               uuid.New().String(),
		Name:              groupName,
		Type:              newGroup.Type,
		OrganisationID:    org.UID,
		Config:            &newGroup.Config,
		LogoURL:           newGroup.LogoURL,
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		RateLimit:         newGroup.RateLimit,
		RateLimitDuration: newGroup.RateLimitDuration,
		DocumentStatus:    datastore.ActiveDocumentStatus,
	}

	err = gs.groupRepo.CreateGroup(ctx, group)
	if err != nil {
		log.WithError(err).Error("failed to create group")
		return nil, nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create group"))
	}

	newAPIKey := &models.APIKey{
		Name: fmt.Sprintf("%s's default key", group.Name),
		Role: auth.Role{
			Type:   auth.RoleSuperUser,
			Groups: []string{group.UID},
		},
	}

	apiKey, keyString, err := NewSecurityService(gs.groupRepo, gs.apiKeyRepo).CreateAPIKey(ctx, newAPIKey)
	if err != nil {
		return nil, nil, err
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name:      apiKey.Name,
			Role:      apiKey.Role,
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt.Time(),
		},
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt.Time(),
		Key:       keyString,
	}

	return group, resp, nil
}

func (gs *GroupService) UpdateGroup(ctx context.Context, group *datastore.Group, update *models.Group) (*datastore.Group, error) {
	err := util.Validate(update)
	if err != nil {
		log.WithError(err).Error("failed to validate group update")
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	group.Name = update.Name
	group.Config = &update.Config
	if !util.IsStringEmpty(update.LogoURL) {
		group.LogoURL = update.LogoURL
	}

	err = gs.groupRepo.UpdateGroup(ctx, group)
	if err != nil {
		log.WithError(err).Error("failed to to update group")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while updating Group"))
	}

	return group, nil
}

func (gs *GroupService) GetGroups(ctx context.Context, filter *datastore.GroupFilter) ([]*datastore.Group, error) {
	groups, err := gs.groupRepo.LoadGroups(ctx, filter.WithNamesTrimmed())
	if err != nil {
		log.WithError(err).Error("failed to load groups")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while fetching Groups"))
	}

	err = gs.FillGroupsStatistics(ctx, groups)
	if err != nil {
		log.WithError(err).Error("failed to fill statistics of group ")
	}

	return groups, nil
}

func (gs *GroupService) FillGroupsStatistics(ctx context.Context, groups []*datastore.Group) error {
	err := gs.groupRepo.FillGroupsStatistics(ctx, groups)
	if err != nil {
		log.WithError(err).Error("failed to count group applications")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to count group statistics"))
	}

	return nil
}

func (gs *GroupService) DeleteGroup(ctx context.Context, id string) error {
	err := gs.groupRepo.DeleteGroup(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to delete group")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to delete group"))
	}

	// TODO(daniel,subomi): is returning http error necessary for these? since the group itself has been deleted
	err = gs.appRepo.DeleteGroupApps(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to delete group apps")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to delete group apps"))
	}

	err = gs.eventRepo.DeleteGroupEvents(ctx, id)
	if err != nil {
		log.WithError(err).Error("failed to delete group events")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to delete group events"))
	}

	return nil
}
