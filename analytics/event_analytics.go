package analytics

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"
)

type EventAnalytics struct {
	eventRepo  datastore.EventRepository
	groupRepo  datastore.GroupRepository
	orgRepo    datastore.OrganisationRepository
	client     AnalyticsClient
	instanceID string
}

func newEventAnalytics(eventRepo datastore.EventRepository, groupRepo datastore.GroupRepository, orgRepo datastore.OrganisationRepository, client AnalyticsClient, instanceID string) *EventAnalytics {
	return &EventAnalytics{
		eventRepo:  eventRepo,
		groupRepo:  groupRepo,
		orgRepo:    orgRepo,
		client:     client,
		instanceID: instanceID,
	}
}

func (ea *EventAnalytics) Track() error {
	return ea.track(PerPage, Page)
}

func (ea *EventAnalytics) track(perPage, page int) error {
	ctx := context.Background()
	orgs, _, err := ea.orgRepo.LoadOrganisationsPaged(ctx, datastore.Pageable{PerPage: perPage, Page: page, Sort: -1})
	if err != nil {
		return err
	}

	if len(orgs) == 0 {
		return nil
	}

	now := time.Now()
	for _, org := range orgs {
		groups, err := ea.groupRepo.LoadGroups(ctx, &datastore.GroupFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation groups")
			continue
		}

		for _, group := range groups {
			filter := &datastore.Filter{
				Group:    group,
				Pageable: datastore.Pageable{PerPage: 20, Page: 1, Sort: -1},
				SearchParams: datastore.SearchParams{
					CreatedAtStart: time.Unix(0, 0).Unix(),
					CreatedAtEnd:   time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC).Unix(),
				},
			}

			_, pagination, err := ea.eventRepo.LoadEventsPaged(ctx, filter)
			if err != nil {
				log.WithError(err).Error("failed to load events paged")
				continue
			}

			err = ea.client.Export(ea.Name(), Event{"Count": pagination.Total, "Project": group.Name, "Organization": org.Name, "instanceID": ea.instanceID})
			if err != nil {
				log.WithError(err).Error("failed to load export metrics")
				continue
			}
		}
	}

	page += 1

	return ea.track(perPage, page)
}

func (ea *EventAnalytics) Name() string {
	return DailyEventCount
}
