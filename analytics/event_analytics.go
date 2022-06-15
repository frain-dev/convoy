package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/sirupsen/logrus"
)

type EventAnalytics struct {
	eventRepo datastore.EventRepository
	groupRepo datastore.GroupRepository
	orgRepo   datastore.OrganisationRepository
	client    AnalyticsClient
	host      string
}

func newEventAnalytics(eventRepo datastore.EventRepository, groupRepo datastore.GroupRepository, orgRepo datastore.OrganisationRepository, client AnalyticsClient, host string) *EventAnalytics {
	return &EventAnalytics{
		eventRepo: eventRepo,
		groupRepo: groupRepo,
		orgRepo:   orgRepo,
		client:    client,
		host:      host,
	}
}

func (ea *EventAnalytics) Track() error {
	ctx := context.Background()
	groups, err := ea.groupRepo.LoadGroups(ctx, &datastore.GroupFilter{})
	if err != nil {
		return err
	}

	for _, group := range groups {
		_, pagination, err := ea.eventRepo.LoadEventsPaged(ctx, group.UID, "", datastore.SearchParams{}, datastore.Pageable{Sort: -1})
		if err != nil {
			log.WithError(err).Error("failed to load events paged")
			continue
		}

		org, err := ea.orgRepo.FetchOrganisationByID(ctx, group.OrganisationID)
		if err != nil {
			log.WithError(err).Error("failed to load fetch organisation")
			continue
		}

		err = ea.client.Export(ea.Name(), Event{"Count": pagination.Total, "Project": group.Name, "Organization": org.Name, "Host": ea.host})
		if err != nil {
			log.WithError(err).Error("failed to load export metrics")
			continue
		}
	}

	return nil
}

func (ea *EventAnalytics) Name() string {
	return DailyEventCount
}
