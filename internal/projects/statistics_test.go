package projects

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestFillProjectsStatistics(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	tests := []struct {
		name              string
		setup             func() *datastore.Project
		wantSubscriptions bool
		wantEndpoints     bool
		wantSources       bool
		wantEvents        bool
		wantErr           bool
	}{
		{
			name: "should_return_false_for_all_when_project_is_empty",
			setup: func() *datastore.Project {
				return seedProject(t, db, org)
			},
			wantSubscriptions: false,
			wantEndpoints:     false,
			wantSources:       false,
			wantEvents:        false,
			wantErr:           false,
		},
		{
			name: "should_return_true_for_endpoints_when_endpoint_exists",
			setup: func() *datastore.Project {
				project := seedProject(t, db, org)
				_ = seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
				return project
			},
			wantSubscriptions: false,
			wantEndpoints:     true,
			wantSources:       false,
			wantEvents:        false,
			wantErr:           false,
		},
		{
			name: "should_return_true_for_events_when_event_exists",
			setup: func() *datastore.Project {
				project := seedProject(t, db, org)
				endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
				_ = seedEvent(t, db, project, endpoint)
				return project
			},
			wantSubscriptions: false,
			wantEndpoints:     true,
			wantSources:       false,
			wantEvents:        true,
			wantErr:           false,
		},
		{
			name: "should_return_true_for_subscriptions_when_subscription_exists",
			setup: func() *datastore.Project {
				project := seedProject(t, db, org)
				endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
				_ = seedSubscription(t, db, project, endpoint)
				return project
			},
			wantSubscriptions: true,
			wantEndpoints:     true,
			wantSources:       false,
			wantEvents:        false,
			wantErr:           false,
		},
		{
			name: "should_return_true_for_sources_when_source_exists",
			setup: func() *datastore.Project {
				project := seedProject(t, db, org)
				seedSource(t, db, project)
				return project
			},
			wantSubscriptions: false,
			wantEndpoints:     false,
			wantSources:       true,
			wantEvents:        false,
			wantErr:           false,
		},
		{
			name: "should_return_true_for_all_when_all_exist",
			setup: func() *datastore.Project {
				project := seedProject(t, db, org)
				endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
				_ = seedEvent(t, db, project, endpoint)
				_ = seedSubscription(t, db, project, endpoint)
				seedSource(t, db, project)
				return project
			},
			wantSubscriptions: true,
			wantEndpoints:     true,
			wantSources:       true,
			wantEvents:        true,
			wantErr:           false,
		},
		{
			name: "should_fail_with_nil_project",
			setup: func() *datastore.Project {
				return nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := tt.setup()

			err := service.FillProjectsStatistics(ctx, project)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, project.Statistics)

			require.Equal(t, tt.wantSubscriptions, project.Statistics.SubscriptionsExist, "SubscriptionsExist mismatch")
			require.Equal(t, tt.wantEndpoints, project.Statistics.EndpointsExist, "EndpointsExist mismatch")
			require.Equal(t, tt.wantSources, project.Statistics.SourcesExist, "SourcesExist mismatch")
			require.Equal(t, tt.wantEvents, project.Statistics.EventsExist, "EventsExist mismatch")
		})
	}
}

func TestFillProjectsStatistics_MultipleResources(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)

	// Create multiple resources of each type
	endpoint1 := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
	endpoint2 := seedEndpoint(t, db, project, datastore.InactiveEndpointStatus)

	_ = seedEvent(t, db, project, endpoint1)
	_ = seedEvent(t, db, project, endpoint2)

	_ = seedSubscription(t, db, project, endpoint1)
	_ = seedSubscription(t, db, project, endpoint2)

	seedSource(t, db, project)
	seedSource(t, db, project)

	// Fill statistics
	err := service.FillProjectsStatistics(ctx, project)
	require.NoError(t, err)
	require.NotNil(t, project.Statistics)

	// Should return true even with multiple resources
	require.True(t, project.Statistics.SubscriptionsExist)
	require.True(t, project.Statistics.EndpointsExist)
	require.True(t, project.Statistics.SourcesExist)
	require.True(t, project.Statistics.EventsExist)
}

func TestFillProjectsStatistics_IgnoresDeletedResources(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)

	// Create and then delete resources
	endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)

	// Delete the project (which cascades to endpoints, events, etc.)
	err := service.DeleteProject(ctx, project.UID)
	require.NoError(t, err)

	// Create a new project to test
	project2 := seedProject(t, db, org)

	// Fill statistics for the new empty project
	err = service.FillProjectsStatistics(ctx, project2)
	require.NoError(t, err)
	require.NotNil(t, project2.Statistics)

	// Should return false for all since no active resources exist
	require.False(t, project2.Statistics.SubscriptionsExist)
	require.False(t, project2.Statistics.EndpointsExist)
	require.False(t, project2.Statistics.SourcesExist)
	require.False(t, project2.Statistics.EventsExist)

	_ = endpoint
}
