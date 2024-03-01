package telemetry

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var ErrInvalidInstanceID = errors.New("invalid instance id provided")

type TotalEventsTracker struct {
	Orgs        []datastore.Organisation
	EventRepo   datastore.EventRepository
	ProjectRepo datastore.ProjectRepository
}

func (te *TotalEventsTracker) track(ctx context.Context, instanceID string) (metric, error) {
	if util.IsStringEmpty(instanceID) {
		return metric{}, ErrInvalidInstanceID
	}

	mt := metric{
		Name:       metricName(DailyEventCount),
		InstanceID: instanceID,
	}

	for _, org := range te.Orgs {
		projects, err := te.ProjectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation projects")
			continue
		}

		for _, project := range projects {
			count, err := te.EventRepo.CountProjectMessages(ctx, project.UID)
			if err != nil {
				log.WithError(err).Error("failed to load events paged")
				continue
			}

			mt.Count += uint64(count)
		}
	}

	return mt, nil
}

type TotalActiveProjectTracker struct {
	Orgs        []datastore.Organisation
	EventRepo   datastore.EventRepository
	ProjectRepo datastore.ProjectRepository
}

func (ta *TotalActiveProjectTracker) track(ctx context.Context, instanceID string) (metric, error) {
	if util.IsStringEmpty(instanceID) {
		return metric{}, ErrInvalidInstanceID
	}

	mt := metric{
		Name:       metricName(DailyActiveProjectCount),
		InstanceID: instanceID,
	}

	now := time.Now()
	for _, org := range ta.Orgs {
		projects, err := ta.ProjectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation projects")
			continue
		}

		for _, project := range projects {
			filter := &datastore.Filter{
				SearchParams: datastore.SearchParams{
					CreatedAtStart: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix(),
					CreatedAtEnd:   time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC).Unix(),
				},
			}

			count, err := ta.EventRepo.CountEvents(ctx, project.UID, filter)
			if err != nil {
				log.WithError(err).Error("failed to load events paged")
				continue
			}

			mt.Count += uint64(count)
		}
	}

	return mt, nil
}
