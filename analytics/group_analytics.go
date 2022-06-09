package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type GroupAnalytics struct {
	groupRepo datastore.GroupRepository
	client    AnalyticsClient
	source    AnalyticsSource
}

func newGroupAnalytics(groupRepo datastore.GroupRepository, client AnalyticsClient, source AnalyticsSource) *GroupAnalytics {
	return &GroupAnalytics{
		groupRepo: groupRepo,
		client:    client,
		source:    source,
	}
}

func (g *GroupAnalytics) Track() error {
	groups, err := g.groupRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	if err != nil {
		return err
	}

	return g.client.Export(g.Name(), Event{"Count": len(groups), "Source": g.source})
}

func (g *GroupAnalytics) Name() string {
	return DailyGroupCount
}
