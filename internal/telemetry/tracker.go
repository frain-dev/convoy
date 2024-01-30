package telemetry

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

const perPage = 50

type TotalEventsTracker struct {
	ConfigRepo  datastore.ConfigurationRepository
	EventRepo   datastore.EventRepository
	ProjectRepo datastore.ProjectRepository
	OrgRepo     datastore.OrganisationRepository
}

func (te *TotalEventsTracker) Track() (metric, error) {
	var mt metric
	ctx := context.Background()
	orgs, pagination, err := te.OrgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: perPage, NextCursor: cursor, Direction: datastore.Next})
	if err != nil {
		return mt, err
	}

	if len(orgs) == 0 && !pagination.HasNextPage {
		return mt, nil
	}

	for _, org := range orgs {
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

			err = te.client.Export(te.Name(), Event{"Count": count, "instanceID": te.instanceID})
			if err != nil {
				log.WithError(err).Error("failed to load export metrics")
				continue
			}
		}
	}

	cursor = pagination.NextPageCursor
	return te.track(perPage, cursor)
}

type TotalActiveProjectTracker struct {
	OrgRepo     datastore.OrganisationRepository
	EventRepo   datastore.EventRepository
	ConfigRepo  datastore.ConfigurationRepository
	ProjectRepo datastore.ProjectRepository
}

func (ta *TotalActiveProjectTracker) Track() (metric, error) {
	return ta.track(PerPage, 0, DefaultCursor)
}

func (ta *TotalActiveProjectTracker) track(perPage, count int, cursor string) error {
	ctx := context.Background()
	orgs, pagination, err := ta.OrgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: perPage, NextCursor: cursor, Direction: datastore.Next})
	if err != nil {
		return err
	}

	if len(orgs) == 0 && !pagination.HasNextPage {
		return a.client.Export(a.Name(), Event{"Count": count, "instanceID": a.instanceID})
	}

	now := time.Now()
	for _, org := range orgs {
		projects, err := ta.ProjectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation projects")
			continue
		}

		for _, project := range projects {
			filter := &datastore.Filter{
				Pageable: datastore.Pageable{PerPage: perPage, NextCursor: cursor, Direction: datastore.Next},
				SearchParams: datastore.SearchParams{
					CreatedAtStart: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix(),
					CreatedAtEnd:   time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC).Unix(),
				},
			}

			events, _, err := ta.EventRepo.LoadEventsPaged(ctx, project.UID, filter)
			if err != nil {
				log.WithError(err).Error("failed to load events paged")
				continue
			}

			if len(events) > 0 {
				count += 1
			}
		}
	}

	cursor = pagination.NextPageCursor
	return ta.track(perPage, count, cursor)
}
