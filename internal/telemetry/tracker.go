package telemetry

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/util"
)

var ErrInvalidInstanceID = errors.New("invalid instance id provided")

type EventsCounter struct{}

func (e *EventsCounter) track(ctx context.Context, instanceID string) (Metric, error) {
	return Metric{
		Name:       metricName(EventCounter),
		Version:    convoy.GetVersion(),
		InstanceID: instanceID,
		Count:      1,
	}, nil
}

type TotalEventsTracker struct {
	Orgs        []datastore.Organisation
	EventRepo   datastore.EventRepository
	ProjectRepo datastore.ProjectRepository
}

func (te *TotalEventsTracker) track(ctx context.Context, instanceID string) (Metric, error) {
	if util.IsStringEmpty(instanceID) {
		return Metric{}, ErrInvalidInstanceID
	}

	mt := Metric{
		Name:       metricName(DailyEventCount),
		InstanceID: instanceID,
		Version:    convoy.GetVersion(),
	}

	for _, org := range te.Orgs {
		projects, err := te.ProjectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation projects")
			continue
		}

		for _, p := range projects {
			count, err := te.EventRepo.CountProjectMessages(ctx, p.UID)
			if err != nil {
				log.WithError(err).Error("failed to load events paged")
				continue
			}
			mt.Count += uint64(count) + uint64(p.RetainedEvents)
		}
	}

	return mt, nil
}

type TotalActiveProjectTracker struct {
	Orgs        []datastore.Organisation
	EventRepo   datastore.EventRepository
	ProjectRepo datastore.ProjectRepository
}

func (ta *TotalActiveProjectTracker) track(ctx context.Context, instanceID string) (Metric, error) {
	if util.IsStringEmpty(instanceID) {
		return Metric{}, ErrInvalidInstanceID
	}

	mt := Metric{
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
