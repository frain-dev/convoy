package projects

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
)

func TestGetProjectsWithEventsInTheInterval(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(database.Database, *datastore.Organisation) []string
		interval  int
		wantCount int
		checkFunc func(*testing.T, []datastore.ProjectEvents)
	}{
		{
			name: "should_return_empty_when_no_events",
			setup: func(db database.Database, org *datastore.Organisation) []string {
				project := seedProject(t, db, org)
				return []string{project.UID}
			},
			interval:  24, // 24 hours
			wantCount: 0,
		},
		{
			name: "should_return_projects_with_events_in_interval",
			setup: func(db database.Database, org *datastore.Organisation) []string {
				project := seedProject(t, db, org)
				endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)

				// Create events within the interval
				_ = seedEvent(t, db, project, endpoint)
				_ = seedEvent(t, db, project, endpoint)
				_ = seedEvent(t, db, project, endpoint)

				return []string{project.UID}
			},
			interval:  24,
			wantCount: 1,
			checkFunc: func(t *testing.T, results []datastore.ProjectEvents) {
				require.Len(t, results, 1)
				require.Equal(t, 3, results[0].EventsCount)
			},
		},
		{
			name: "should_return_multiple_projects_ordered_by_event_count",
			setup: func(db database.Database, org *datastore.Organisation) []string {
				// Project 1 with 5 events
				project1 := seedProject(t, db, org)
				endpoint1 := seedEndpoint(t, db, project1, datastore.ActiveEndpointStatus)
				for i := 0; i < 5; i++ {
					_ = seedEvent(t, db, project1, endpoint1)
				}

				// Project 2 with 3 events
				project2 := seedProject(t, db, org)
				endpoint2 := seedEndpoint(t, db, project2, datastore.ActiveEndpointStatus)
				for i := 0; i < 3; i++ {
					_ = seedEvent(t, db, project2, endpoint2)
				}

				// Project 3 with 10 events
				project3 := seedProject(t, db, org)
				endpoint3 := seedEndpoint(t, db, project3, datastore.ActiveEndpointStatus)
				for i := 0; i < 10; i++ {
					_ = seedEvent(t, db, project3, endpoint3)
				}

				return []string{project1.UID, project2.UID, project3.UID}
			},
			interval:  24,
			wantCount: 3,
			checkFunc: func(t *testing.T, results []datastore.ProjectEvents) {
				require.Len(t, results, 3)

				// Should be ordered by event count descending
				require.Equal(t, 10, results[0].EventsCount)
				require.Equal(t, 5, results[1].EventsCount)
				require.Equal(t, 3, results[2].EventsCount)
			},
		},
		{
			name: "should_only_include_events_within_interval",
			setup: func(db database.Database, org *datastore.Organisation) []string {
				project := seedProject(t, db, org)
				endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)

				// Create recent events (should be included)
				_ = seedEvent(t, db, project, endpoint)
				_ = seedEvent(t, db, project, endpoint)

				return []string{project.UID}
			},
			interval:  1, // 1 hour
			wantCount: 1,
			checkFunc: func(t *testing.T, results []datastore.ProjectEvents) {
				require.Len(t, results, 1)
				require.GreaterOrEqual(t, results[0].EventsCount, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, ctx := setupTestDB(t)
			defer db.Close()

			service := New(testLogger, db)
			org := seedOrganisation(t, db)

			projectIDs := tt.setup(db, org)

			results, err := service.GetProjectsWithEventsInTheInterval(ctx, tt.interval)
			require.NoError(t, err)
			require.NotNil(t, results)

			if tt.wantCount == 0 {
				require.Empty(t, results)
			} else {
				require.NotEmpty(t, results)

				// Verify returned project IDs are from our setup
				for _, result := range results {
					found := false
					for _, id := range projectIDs {
						if result.Id == id {
							found = true
							break
						}
					}
					require.True(t, found, "Returned project %s was not in setup", result.Id)
				}

				if tt.checkFunc != nil {
					tt.checkFunc(t, results)
				}
			}
		})
	}
}

func TestGetProjectsWithEventsInTheInterval_DifferentIntervals(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)
	endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)

	// Create some events
	for i := 0; i < 5; i++ {
		_ = seedEvent(t, db, project, endpoint)
	}

	// Test different intervals
	intervals := []int{1, 6, 12, 24, 48, 72}

	for _, interval := range intervals {
		t.Run("interval_"+string(rune(interval))+"_hours", func(t *testing.T) {
			results, err := service.GetProjectsWithEventsInTheInterval(ctx, interval)
			require.NoError(t, err)
			require.NotEmpty(t, results)

			// Should find the project with events
			found := false
			for _, result := range results {
				if result.Id == project.UID {
					found = true
					require.Equal(t, 5, result.EventsCount)
					break
				}
			}
			require.True(t, found, "Project not found in interval %d", interval)
		})
	}
}

func TestCountProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Initial count should be 0
	count, err := service.CountProjects(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, int64(0))

	initialCount := count

	// Create projects
	_ = seedProject(t, db, org)
	_ = seedProject(t, db, org)
	_ = seedProject(t, db, org)

	// Count should increase by 3
	count, err = service.CountProjects(ctx)
	require.NoError(t, err)
	require.Equal(t, initialCount+3, count)
}

func TestCountProjects_AfterDeletion(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Get initial count
	initialCount, err := service.CountProjects(ctx)
	require.NoError(t, err)

	// Create a project
	project := seedProject(t, db, org)

	// Count should increase
	count, err := service.CountProjects(ctx)
	require.NoError(t, err)
	require.Equal(t, initialCount+1, count)

	// Delete the project
	err = service.DeleteProject(ctx, project.UID)
	require.NoError(t, err)

	// Count should go back to initial (soft deleted projects are not counted)
	count, err = service.CountProjects(ctx)
	require.NoError(t, err)
	require.Equal(t, initialCount, count)
}

func TestCountProjects_MultipleOrganisations(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org1 := seedOrganisation(t, db)
	org2 := seedOrganisation(t, db)

	initialCount, err := service.CountProjects(ctx)
	require.NoError(t, err)

	// Create projects for different orgs
	_ = seedProject(t, db, org1)
	_ = seedProject(t, db, org1)
	_ = seedProject(t, db, org2)
	_ = seedProject(t, db, org2)
	_ = seedProject(t, db, org2)

	// Total count should include all projects across all orgs
	count, err := service.CountProjects(ctx)
	require.NoError(t, err)
	require.Equal(t, initialCount+5, count)
}
