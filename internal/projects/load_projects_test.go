package projects

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestLoadProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org1 := seedOrganisation(t, db)
	org2 := seedOrganisation(t, db)

	// Create multiple projects for org1
	project1 := seedProject(t, db, org1)
	project2 := seedProject(t, db, org1)

	// Create a project for org2
	project3 := seedProject(t, db, org2)

	tests := []struct {
		name       string
		filter     *datastore.ProjectFilter
		wantCount  int
		wantOrgID  string
		shouldFind []string
	}{
		{
			name: "should_load_all_projects_for_org1",
			filter: &datastore.ProjectFilter{
				OrgID: org1.UID,
			},
			wantCount:  2,
			wantOrgID:  org1.UID,
			shouldFind: []string{project1.UID, project2.UID},
		},
		{
			name: "should_load_all_projects_for_org2",
			filter: &datastore.ProjectFilter{
				OrgID: org2.UID,
			},
			wantCount:  1,
			wantOrgID:  org2.UID,
			shouldFind: []string{project3.UID},
		},
		{
			name: "should_load_all_projects_with_empty_filter",
			filter: &datastore.ProjectFilter{
				OrgID: "",
			},
			wantCount: 3,
		},
		{
			name:      "should_load_all_projects_with_nil_filter",
			filter:    nil,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projects, err := service.LoadProjects(ctx, tt.filter)
			require.NoError(t, err)
			require.NotNil(t, projects)
			require.Len(t, projects, tt.wantCount)

			// If specific org ID is expected, verify all projects belong to that org
			if tt.wantOrgID != "" {
				for _, p := range projects {
					require.Equal(t, tt.wantOrgID, p.OrganisationID)

					// Verify config is loaded
					require.NotNil(t, p.Config)
					require.NotNil(t, p.Config.RateLimit)
					require.NotNil(t, p.Config.Strategy)
				}
			}

			// If specific project IDs should be found, verify them
			if len(tt.shouldFind) > 0 {
				foundIDs := make(map[string]bool)
				for _, p := range projects {
					foundIDs[p.UID] = true
				}

				for _, expectedID := range tt.shouldFind {
					require.True(t, foundIDs[expectedID], "Expected to find project %s", expectedID)
				}
			}
		})
	}
}

func TestLoadProjects_Ordering(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Create projects in specific order
	project1 := seedProject(t, db, org)
	project2 := seedProject(t, db, org)
	project3 := seedProject(t, db, org)

	filter := &datastore.ProjectFilter{OrgID: org.UID}
	projects, err := service.LoadProjects(ctx, filter)
	require.NoError(t, err)
	require.Len(t, projects, 3)

	// Verify projects are ordered by ID (as per SQL ORDER BY p.id)
	require.True(t, projects[0].UID <= projects[1].UID)
	require.True(t, projects[1].UID <= projects[2].UID)

	// Verify all expected projects are present
	projectIDs := make(map[string]bool)
	for _, p := range projects {
		projectIDs[p.UID] = true
	}
	require.True(t, projectIDs[project1.UID])
	require.True(t, projectIDs[project2.UID])
	require.True(t, projectIDs[project3.UID])
}

func TestLoadProjects_EmptyResult(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	// Don't create any projects for this org
	filter := &datastore.ProjectFilter{OrgID: org.UID}
	projects, err := service.LoadProjects(ctx, filter)
	require.NoError(t, err)
	require.NotNil(t, projects)
	require.Empty(t, projects)
}
