package analytics

import (
	"context"
	"time"

	"github.com/frain-dev/convoy/datastore"
)

type ActiveGroupAnalytics struct {
	groupRepo datastore.GroupRepository
	eventRepo datastore.EventRepository
	client    AnalyticsClient
}

func newActiveGroupAnalytics(groupRepo datastore.GroupRepository, eventRepo datastore.EventRepository, client AnalyticsClient) *ActiveGroupAnalytics {
	return &ActiveGroupAnalytics{groupRepo: groupRepo, eventRepo: eventRepo, client: client}
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
			return err
		}

		if len(events) > 0 {
			count += 1
		}
	}

	return a.client.Export(a.Name(), Event{"Count": count})

}

func (a *ActiveGroupAnalytics) Name() string {
	return DailyActiveGroupCount
}
