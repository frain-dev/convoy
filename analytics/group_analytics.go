package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type GroupAnalytics struct {
	groupRepo datastore.GroupRepository
	client    AnalyticsClient
	host      string
}

func newGroupAnalytics(groupRepo datastore.GroupRepository, client AnalyticsClient, host string) *GroupAnalytics {
	return &GroupAnalytics{
		groupRepo: groupRepo,
		client:    client,
		host:      host,
	}
}

func (g *GroupAnalytics) Track() error {
	groups, err := g.groupRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	if err != nil {
		return err
	}

	return g.client.Export(g.Name(), Event{"Count": len(groups), "Host": g.host})
}

func (g *GroupAnalytics) Name() string {
	return DailyGroupCount
}
