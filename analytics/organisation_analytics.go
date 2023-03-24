package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type OrganisationAnalytics struct {
	orgRepo    datastore.OrganisationRepository
	client     AnalyticsClient
	instanceID string
}

func newOrganisationAnalytics(orgRepo datastore.OrganisationRepository, client AnalyticsClient, instanceID string) *OrganisationAnalytics {
	return &OrganisationAnalytics{
		orgRepo:    orgRepo,
		client:     client,
		instanceID: instanceID,
	}
}

func (o *OrganisationAnalytics) Track() error {
	return o.track(PerPage, 0, DefaultCursor)
}

func (o *OrganisationAnalytics) track(perPage, count int, cursor string) error {
	orgs, pagination, err := o.orgRepo.LoadOrganisationsPaged(context.Background(), datastore.Pageable{
		PerPage:    perPage,
		NextCursor: cursor,
		Direction:  datastore.Next,
	})
	if err != nil {
		return err
	}

	if len(orgs) == 0 && !pagination.HasNextPage {
		return o.client.Export(o.Name(), Event{"Count": count, "instanceID": o.instanceID})
	}

	count += len(orgs)
	cursor = pagination.NextPageCursor
	return o.track(perPage, count, cursor)
}

func (o *OrganisationAnalytics) Name() string {
	return DailyOrganisationCount
}
