package analytics

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type ActiveProjectAnalytics struct {
	projectRepo datastore.ProjectRepository
	eventRepo   datastore.EventRepository
	orgRepo     datastore.OrganisationRepository
	client      AnalyticsClient
	instanceID  string
}

func newActiveProjectAnalytics(projectRepo datastore.ProjectRepository, eventRepo datastore.EventRepository, orgRepo datastore.OrganisationRepository, client AnalyticsClient, instanceID string) *ActiveProjectAnalytics {
	return &ActiveProjectAnalytics{
		projectRepo: projectRepo,
		eventRepo:   eventRepo,
		orgRepo:     orgRepo,
		client:      client,
		instanceID:  instanceID,
	}
}

func (a *ActiveProjectAnalytics) Track() error {
	return a.track(PerPage, Page, DefaultCursor)
}

func (a *ActiveProjectAnalytics) track(perPage, count int, cursor string) error {
	ctx := context.Background()
	orgs, pagination, err := a.orgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: perPage, NextCursor: cursor, Direction: datastore.Next})
	if err != nil {
		return err
	}

	if len(orgs) == 0 && !pagination.HasNextPage {
		return a.client.Export(a.Name(), Event{"Count": count, "instanceID": a.instanceID})
	}

	now := time.Now()
	for _, org := range orgs {
		projects, err := a.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation projects")
			continue
		}

		for _, project := range projects {
			filter := &datastore.Filter{
				Pageable: datastore.Pageable{},
				SearchParams: datastore.SearchParams{
					CreatedAtStart: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix(),
					CreatedAtEnd:   time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC).Unix(),
				},
			}

			events, _, err := a.eventRepo.LoadEventsPaged(ctx, project.UID, filter)
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
	return a.track(perPage, count, cursor)
}

func (a *ActiveProjectAnalytics) Name() string {
	return DailyActiveProjectCount
}
