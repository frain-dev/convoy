package analytics

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

type ProjectAnalytics struct {
	projectRepo datastore.ProjectRepository
	client      AnalyticsClient
	instanceID  string
}

func newProjectAnalytics(projectRepo datastore.ProjectRepository, client AnalyticsClient, instanceID string) *ProjectAnalytics {
	return &ProjectAnalytics{
		projectRepo: projectRepo,
		client:      client,
		instanceID:  instanceID,
	}
}

func (g *ProjectAnalytics) Track() error {
	groups, err := g.projectRepo.LoadProjects(context.Background(), &datastore.GroupFilter{})
	if err != nil {
		return err
	}

	return g.client.Export(g.Name(), Event{"Count": len(groups), "instanceID": g.instanceID})
}

func (g *ProjectAnalytics) Name() string {
	return DailyGroupCount
}
