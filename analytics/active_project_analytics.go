package analytics

import (
	"context"
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
	return a.track(PerPage, Page, 0)
}

func (a *ActiveProjectAnalytics) track(perPage, page, count int) error {
	ctx := context.Background()
	orgs, _, err := a.orgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: perPage, Page: page, Sort: -1})
	if err != nil {
		return err
	}

	if len(orgs) == 0 {
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
				Pageable: datastore.Pageable{Sort: -1, PerPage: 1, Page: 1},
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

	page += 1

	return a.track(perPage, page, count)
}

func (a *ActiveProjectAnalytics) Name() string {
	return DailyActiveProjectCount
}
