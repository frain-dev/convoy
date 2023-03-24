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
	_, pagination, err := o.orgRepo.LoadOrganisationsPaged(context.Background(), datastore.Pageable{})
	if err != nil {
		return err
	}

	return o.client.Export(o.Name(), Event{"Count": pagination.NextPageCursor, "instanceID": o.instanceID})
}

func (o *OrganisationAnalytics) Name() string {
	return DailyOrganisationCount
}
