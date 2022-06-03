package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type GroupAnalytics struct {
	groupRepo datastore.GroupRepository
	client    AnalyticsClient
}

func newGroupAnalytics(groupRepo datastore.GroupRepository, client AnalyticsClient) *GroupAnalytics {
	return &GroupAnalytics{groupRepo: groupRepo, client: client}
}

func (g *GroupAnalytics) Track() error {
	groups, err := g.groupRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	if err != nil {
		return err
	}

	return g.client.Export(g.Name(), Event{"Count": len(groups)})
}

func (g *GroupAnalytics) Name() string {
	return DailyGroupCount
}
