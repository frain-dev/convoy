package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type EventAnalytics struct {
	eventRepo datastore.EventRepository
	client    AnalyticsClient
}

func NewEventAnalytics(eventRepo datastore.EventRepository, client AnalyticsClient) *EventAnalytics {
	return &EventAnalytics{eventRepo: eventRepo, client: client}
}

func (ea *EventAnalytics) Track() error {
	_, pagination, err := ea.eventRepo.LoadEventsPaged(context.Background(), "", "", datastore.SearchParams{}, datastore.Pageable{Sort: -1})
	if err != nil {
		return err
	}

	return ea.client.Export(ea.Name(), Event{"Count": pagination.Total})
}

func (ea *EventAnalytics) Name() string {
	return DailyEventCount
}
