package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GroupService struct {
	appRepo           datastore.ApplicationRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	limiter           limiter.RateLimiter
	cache             cache.Cache
}

func NewGroupService(appRepo datastore.ApplicationRepository, groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, limiter limiter.RateLimiter, cache cache.Cache) *GroupService {
	return &GroupService{
		appRepo:           appRepo,
		groupRepo:         groupRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		limiter:           limiter,
		cache:             cache,
	}
}

func (gs *GroupService) CreateGroup(ctx context.Context, newGroup *models.Group) (*datastore.Group, error) {
	err := util.Validate(newGroup)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	groupName := newGroup.Name

	config := newGroup.Config
	if newGroup.Config == nil {
		config = &datastore.GroupConfig{}
		config.Signature = &datastore.DefaultSignatureConfig
		config.Strategy = &datastore.DefaultStrategyConfig
		config.RateLimit = &datastore.DefaultRateLimitConfig
	} else {
		if newGroup.Config.Signature == nil {
			config.Signature = &datastore.DefaultSignatureConfig
		}

		if newGroup.Config.Strategy == nil {
			config.Strategy = &datastore.DefaultStrategyConfig
		}

		if newGroup.Config.RateLimit == nil {
			config.RateLimit = &datastore.DefaultRateLimitConfig
		}
	}

	if newGroup.RateLimit == 0 {
		newGroup.RateLimit = convoy.RATE_LIMIT
	}

	if util.IsStringEmpty(newGroup.RateLimitDuration) {
		newGroup.RateLimitDuration = convoy.RATE_LIMIT_DURATION
	}

	group := &datastore.Group{
		UID:               uuid.New().String(),
		Name:              groupName,
		Type:              newGroup.Type,
		Config:            config,
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

		if err == datastore.ErrDuplicateGroupName {
			return nil, NewServiceError(http.StatusBadRequest, err)
		}

		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create group"))
	}

	return group, nil
}

func (gs *GroupService) UpdateGroup(ctx context.Context, group *datastore.Group, update *models.UpdateGroup) (*datastore.Group, error) {
	err := util.Validate(update)
	if err != nil {
		log.WithError(err).Error("failed to validate group update")
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	if !util.IsStringEmpty(update.Name) {
		group.Name = update.Name
	}

	if update.Config != nil {
		group.Config = update.Config
	}

	if !util.IsStringEmpty(update.LogoURL) {
		group.LogoURL = update.LogoURL
	}

	err = gs.groupRepo.UpdateGroup(ctx, group)
	if err != nil {
		log.WithError(err).Error("failed to to update group")
		return nil, NewServiceError(http.StatusBadRequest, err)
	}

	groupCacheKey := convoy.GroupsCacheKey.Get(group.UID).String()
	err = gs.cache.Set(ctx, groupCacheKey, &group, time.Minute*5)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
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
