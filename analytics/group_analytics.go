package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type GroupAnalytics struct {
	groupRepo  datastore.GroupRepository
	client     AnalyticsClient
	instanceID string
}

func newGroupAnalytics(groupRepo datastore.GroupRepository, client AnalyticsClient, instanceID string) *GroupAnalytics {
	return &GroupAnalytics{
		groupRepo:  groupRepo,
		client:     client,
		instanceID: instanceID,
	}
}

func (g *GroupAnalytics) Track() error {
	groups, err := g.groupRepo.LoadGroups(context.Background(), &datastore.GroupFilter{})
	if err != nil {
		return err
	}

	return g.client.Export(g.Name(), Event{"Count": len(groups), "instanceID": g.instanceID})
}

func (g *GroupAnalytics) Name() string {
	return DailyGroupCount
}
