package analytics

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"
)

type ActiveGroupAnalytics struct {
	groupRepo  datastore.GroupRepository
	eventRepo  datastore.EventRepository
	client     AnalyticsClient
	instanceID string
}

func newActiveGroupAnalytics(groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, client AnalyticsClient, instanceID string) *ActiveGroupAnalytics {
	return &ActiveGroupAnalytics{
		groupRepo:  groupRepo,
		eventRepo:  eventRepo,
		client:     client,
		instanceID: instanceID,
	}

}

func (a *ActiveGroupAnalytics) Track() error {
	groups, err := a.groupRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	if err != nil {
		return err
	}

	count := 0
	now := time.Now()
	for _, group := range groups {
		filter := datastore.SearchParams{
			CreatedAtStart: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix(),
			CreatedAtEnd:   time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC).Unix(),
		}

		events, _, err := a.eventRepo.LoadEventsPaged(context.Background(), group.UID, "", filter, datastore.Pageable{Sort: -1})
		if err != nil {
			log.WithError(err).Error("failed to load events paged")
			continue
		}

		if len(events) > 0 {
			count += 1
		}
	}

	return a.client.Export(a.Name(), Event{"Count": count, "instanceID": a.instanceID})

}

func (a *ActiveGroupAnalytics) Name() string {
	return DailyActiveGroupCount
}
