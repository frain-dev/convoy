package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type OrganisationAnalytics struct {
	orgRepo datastore.OrganisationRepository
	client  AnalyticsClient
	host    string
}

func newOrganisationAnalytics(orgRepo datastore.OrganisationRepository, client AnalyticsClient, host string) *OrganisationAnalytics {
	return &OrganisationAnalytics{
		orgRepo: orgRepo,
		client:  client,
		host:    host,
	}
}

func (o *OrganisationAnalytics) Track() error {
	_, pagination, err := o.orgRepo.LoadOrganisationsPaged(context.Background(), datastore.Pageable{Sort: -1})
	if err != nil {
		return err
	}

	return o.client.Export(o.Name(), Event{"Count": pagination.Total, "Host": o.host})
}

func (o *OrganisationAnalytics) Name() string {
	return DailyOrganisationCount
}
