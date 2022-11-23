package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/auth"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GroupService struct {
	apiKeyRepo        datastore.APIKeyRepository
	appRepo           datastore.ApplicationRepository
	groupRepo         datastore.GroupRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	limiter           limiter.RateLimiter
	cache             cache.Cache
}

func NewGroupService(apiKeyRepo datastore.APIKeyRepository, appRepo datastore.ApplicationRepository, groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, limiter limiter.RateLimiter, cache cache.Cache) *GroupService {
	return &GroupService{
		apiKeyRepo:        apiKeyRepo,
		appRepo:           appRepo,
		groupRepo:         groupRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		limiter:           limiter,
		cache:             cache,
	}
}

func (gs *GroupService) CreateGroup(ctx context.Context, newGroup *models.Group, org *datastore.Organisation, member *datastore.OrganisationMember) (*datastore.Group, *models.APIKeyResponse, error) {
	err := util.Validate(newGroup)
	if err != nil {
		return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	groupName := newGroup.Name

	config := newGroup.Config
	if newGroup.Config == nil {
		config = &datastore.GroupConfig{}
		config.Signature = datastore.GetDefaultSignatureConfig()
		config.Strategy = &datastore.DefaultStrategyConfig
		config.RateLimit = &datastore.DefaultRateLimitConfig
		config.RetentionPolicy = &datastore.DefaultRetentionPolicy
	} else {
		if newGroup.Config.Signature == nil {
			config.Signature = datastore.GetDefaultSignatureConfig()
		}

		checkSignatureVersions(newGroup.Config.Signature.Versions)

		if newGroup.Config.Strategy == nil {
			config.Strategy = &datastore.DefaultStrategyConfig
		}

		if newGroup.Config.RateLimit == nil {
			config.RateLimit = &datastore.DefaultRateLimitConfig
		}

		if newGroup.Config.RetentionPolicy == nil {
			config.RetentionPolicy = &datastore.DefaultRetentionPolicy
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
		OrganisationID:    org.UID,
		Config:            config,
		LogoURL:           newGroup.LogoURL,
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		RateLimit:         newGroup.RateLimit,
		RateLimitDuration: newGroup.RateLimitDuration,
	}

	err = gs.groupRepo.CreateGroup(ctx, group)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to create group")
		if err == datastore.ErrDuplicateGroupName {
			return nil, nil, util.NewServiceError(http.StatusBadRequest, err)
		}

		return nil, nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to create group"))
	}

	newAPIKey := &models.APIKey{
		Name: fmt.Sprintf("%s's default key", group.Name),
		Role: models.Role{
			Type:  auth.RoleAdmin,
			Group: group.UID,
		},
	}

	apiKey, keyString, err := NewSecurityService(gs.groupRepo, gs.apiKeyRepo).CreateAPIKey(ctx, member, newAPIKey)
	if err != nil {
		return nil, nil, err
	}

	resp := &models.APIKeyResponse{
		APIKey: models.APIKey{
			Name: apiKey.Name,
			Role: models.Role{
				Type:  apiKey.Role.Type,
				Group: apiKey.Role.Group,
			},
			Type:      apiKey.Type,
			ExpiresAt: apiKey.ExpiresAt.Time(),
		},
		UID:       apiKey.UID,
		CreatedAt: apiKey.CreatedAt.Time(),
		Key:       keyString,
	}

	return group, resp, nil
}

func (gs *GroupService) UpdateGroup(ctx context.Context, group *datastore.Group, update *models.UpdateGroup) (*datastore.Group, error) {
	err := util.Validate(update)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to validate group update")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	if !util.IsStringEmpty(update.Name) {
		group.Name = update.Name
	}

	if update.Config != nil {
		group.Config = update.Config
		checkSignatureVersions(group.Config.Signature.Versions)
	}

	if !util.IsStringEmpty(update.LogoURL) {
		group.LogoURL = update.LogoURL
	}

	err = gs.groupRepo.UpdateGroup(ctx, group)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to to update group")
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	groupCacheKey := convoy.GroupsCacheKey.Get(group.UID).String()
	err = gs.cache.Set(ctx, groupCacheKey, &group, time.Minute*5)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	return group, nil
}

func checkSignatureVersions(versions []datastore.SignatureVersion) {
	for i := range versions {
		v := &versions[i]
		if v.UID == "" {
			v.UID = uuid.NewString()
		}

		if v.CreatedAt == 0 {
			v.CreatedAt = primitive.NewDateTimeFromTime(time.Now())
		}
	}
}

func (gs *GroupService) GetGroups(ctx context.Context, filter *datastore.GroupFilter) ([]*datastore.Group, error) {
	groups, err := gs.groupRepo.LoadGroups(ctx, filter.WithNamesTrimmed())
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to load groups")
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("an error occurred while fetching Groups"))
	}

	err = gs.FillGroupsStatistics(ctx, groups)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to fill statistics of group ")
	}

	return groups, nil
}

func (gs *GroupService) FillGroupsStatistics(ctx context.Context, groups []*datastore.Group) error {
	err := gs.groupRepo.FillGroupsStatistics(ctx, groups)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to count group applications")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to count group statistics"))
	}

	return nil
}

func (gs *GroupService) DeleteGroup(ctx context.Context, id string) error {
	err := gs.groupRepo.DeleteGroup(ctx, id)
	if err != nil {
		log.FromContext(ctx).WithError(err).Error("failed to delete group")
		return util.NewServiceError(http.StatusBadRequest, errors.New("failed to delete group"))
	}

	return nil
}
