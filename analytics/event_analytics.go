package analytics

import (
	"context"
	"fmt"
	"math"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type EventAnalytics struct {
	eventRepo   datastore.EventRepository
	projectRepo datastore.ProjectRepository
	orgRepo     datastore.OrganisationRepository
	client      AnalyticsClient
	instanceID  string
}

func newEventAnalytics(eventRepo datastore.EventRepository, projectRepo datastore.ProjectRepository, orgRepo datastore.OrganisationRepository, client AnalyticsClient, instanceID string) *EventAnalytics {
	return &EventAnalytics{
		eventRepo:   eventRepo,
		projectRepo: projectRepo,
		orgRepo:     orgRepo,
		client:      client,
		instanceID:  instanceID,
	}
}

func (ea *EventAnalytics) Track() error {
	return ea.track(PerPage, fmt.Sprintf("%d", math.MaxInt))
}

func (ea *EventAnalytics) track(perPage int, cursor string) error {
	ctx := context.Background()
	orgs, pagination, err := ea.orgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: perPage, NextCursor: cursor, Direction: datastore.Next})
	if err != nil {
		return err
	}

	if len(orgs) == 0 && !pagination.HasNextPage {
		return nil
	}

	for _, org := range orgs {
		projects, err := ea.projectRepo.LoadProjects(ctx, &datastore.ProjectFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation projects")
			continue
		}

		for _, project := range projects {
			count, err := ea.eventRepo.CountProjectMessages(ctx, project.UID)
			if err != nil {
				log.WithError(err).Error("failed to load events paged")
				continue
			}

			err = ea.client.Export(ea.Name(), Event{"Count": count, "Project": project.Name, "Organization": org.Name, "instanceID": ea.instanceID})
			if err != nil {
				log.WithError(err).Error("failed to load export metrics")
				continue
			}
		}
	}

	cursor = pagination.NextPageCursor
	return ea.track(perPage, cursor)
}

func (ea *EventAnalytics) Name() string {
	return DailyEventCount
}
