package analytics

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

type ActiveGroupAnalytics struct {
	groupRepo  datastore.GroupRepository
	eventRepo  datastore.EventRepository
	orgRepo    datastore.OrganisationRepository
	client     AnalyticsClient
	instanceID string
}

func newActiveGroupAnalytics(groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, orgRepo datastore.OrganisationRepository, client AnalyticsClient, instanceID string) *ActiveGroupAnalytics {
	return &ActiveGroupAnalytics{
		groupRepo:  groupRepo,
		eventRepo:  eventRepo,
		orgRepo:    orgRepo,
		client:     client,
		instanceID: instanceID,
	}
}

func (a *ActiveGroupAnalytics) Track() error {
	return a.track(PerPage, Page, 0)
}

func (a *ActiveGroupAnalytics) track(perPage, page, count int) error {
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
		groups, err := a.groupRepo.LoadGroups(ctx, &datastore.GroupFilter{OrgID: org.UID})
		if err != nil {
			log.WithError(err).Error("failed to load organisation groups")
			continue
		}

		for _, group := range groups {
			filter := &datastore.Filter{
				Group:    group,
				Pageable: datastore.Pageable{Sort: -1, PerPage: 1, Page: 1},
				SearchParams: datastore.SearchParams{
					CreatedAtStart: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix(),
					CreatedAtEnd:   time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC).Unix(),
				},
			}

			events, _, err := a.eventRepo.LoadEventsPaged(ctx, filter)
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

func (a *ActiveGroupAnalytics) Name() string {
	return DailyActiveGroupCount
}
