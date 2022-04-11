package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/limiter"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
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
}

func NewGroupService(appRepo datastore.ApplicationRepository, groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, eventDeliveryRepo datastore.EventDeliveryRepository, limiter limiter.RateLimiter) *GroupService {
	return &GroupService{
		appRepo:           appRepo,
		groupRepo:         groupRepo,
		eventRepo:         eventRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		limiter:           limiter,
	}
}

func (gs *GroupService) CreateGroup(ctx context.Context, newGroup *models.Group) (*datastore.Group, error) {
	groupName := newGroup.Name
	err := util.Validate(newGroup)
	if err != nil {
		return nil, NewServiceError(http.StatusBadRequest, err)
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
		return nil, NewServiceError(http.StatusBadRequest, errors.New("failed to create group"))
	}

	// register task.
	taskName := convoy.EventProcessor.SetPrefix(groupName)
	task.CreateTask(taskName, *group, task.ProcessEventDelivery(gs.appRepo, gs.eventDeliveryRepo, gs.groupRepo, gs.limiter))

	return group, nil
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
	groups, err := gs.groupRepo.LoadGroups(ctx, filter)
	if err != nil {
		log.WithError(err).Error("failed to load groups")
		return nil, NewServiceError(http.StatusBadRequest, errors.New("an error occurred while fetching Groups"))
	}

	for _, group := range groups {
		err = gs.FillGroupStatistics(ctx, group)
		if err != nil {
			log.WithError(err).Errorf("failed to fill statistics of group %s", group.UID)
		}
	}
	return groups, nil
}

func (gs *GroupService) FillGroupStatistics(ctx context.Context, g *datastore.Group) error {
	appCount, err := gs.appRepo.CountGroupApplications(ctx, g.UID)
	if err != nil {
		log.WithError(err).Error("failed to count group applications")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to count group statistics"))
	}

	msgCount, err := gs.eventRepo.CountGroupMessages(ctx, g.UID)
	if err != nil {
		log.WithError(err).Error("failed to count group messages")
		return NewServiceError(http.StatusBadRequest, errors.New("failed to count group statistics"))
	}

	g.Statistics = &datastore.GroupStatistics{
		MessagesSent: msgCount,
		TotalApps:    appCount,
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
