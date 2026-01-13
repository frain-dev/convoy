package organisation_members

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
)

func Test_FindUserProjects_SingleProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project := seedProject(t, db, org.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Find user projects
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, project.UID, projects[0].UID)
	require.Equal(t, org.UID, projects[0].OrganisationID)
}

func Test_FindUserProjects_MultipleProjects_SingleOrg(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project1 := seedProject(t, db, org.UID)
	project2 := seedProject(t, db, org.UID)
	project3 := seedProject(t, db, org.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Find user projects
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Len(t, projects, 3)

	// Verify all project IDs are present
	projectIDs := make(map[string]bool)
	for _, p := range projects {
		projectIDs[p.UID] = true
	}
	require.True(t, projectIDs[project1.UID])
	require.True(t, projectIDs[project2.UID])
	require.True(t, projectIDs[project3.UID])
}

func Test_FindUserProjects_MultipleOrgs(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user is member of 2 orgs
	user := seedUser(t, db, "multi-org@example.com")
	org1 := seedOrganisation(t, db, user.UID)
	org2 := seedOrganisation(t, db, user.UID)

	project1 := seedProject(t, db, org1.UID)
	project2 := seedProject(t, db, org2.UID)

	_ = seedOrganisationMember(t, db, org1.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})
	_ = seedOrganisationMember(t, db, org2.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Find user projects
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Len(t, projects, 2)

	// Verify projects from both orgs
	projectIDs := make(map[string]bool)
	for _, p := range projects {
		projectIDs[p.UID] = true
	}
	require.True(t, projectIDs[project1.UID])
	require.True(t, projectIDs[project2.UID])
}

func Test_FindUserProjects_NoProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user with org but no projects
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Find user projects
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Empty(t, projects)
}

func Test_FindUserProjects_NotAMember(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data - user not a member of any org
	user := seedUser(t, db, "")

	// Find user projects
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Empty(t, projects)
}

func Test_FindUserProjects_ExcludesDeletedMembers(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project := seedProject(t, db, org.UID)
	member := seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete member
	err := service.DeleteOrganisationMember(ctx, member.UID, org.UID)
	require.NoError(t, err)

	// Find user projects should return empty
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Empty(t, projects)
	require.NotContains(t, projects, project)
}

func Test_FindUserProjects_ExcludesDeletedProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project1 := seedProject(t, db, org.UID)
	project2 := seedProject(t, db, org.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Delete one project
	err := projectRepo.DeleteProject(ctx, project1.UID)
	require.NoError(t, err)

	// Find user projects should exclude deleted project
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, project2.UID, projects[0].UID)
}

func Test_FindUserProjects_DifferentProjectTypes(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	project1 := seedProject(t, db, org.UID) // OutgoingProject by default
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Find user projects
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	require.Equal(t, project1.UID, projects[0].UID)
	require.Equal(t, datastore.OutgoingProject, projects[0].Type)
}

func Test_FindUserProjects_VerifyProjectFields(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createOrgMemberService(t, db)

	// Seed data
	user := seedUser(t, db, "")
	org := seedOrganisation(t, db, user.UID)
	_ = seedProject(t, db, org.UID)
	_ = seedOrganisationMember(t, db, org.UID, user.UID, auth.Role{Type: auth.RoleProjectAdmin})

	// Find user projects
	projects, err := service.FindUserProjects(ctx, user.UID)
	require.NoError(t, err)
	require.Len(t, projects, 1)

	// Verify all project fields are populated
	p := projects[0]
	require.NotEmpty(t, p.UID)
	require.NotEmpty(t, p.Name)
	require.Equal(t, org.UID, p.OrganisationID)
	require.NotZero(t, p.CreatedAt)
	require.NotZero(t, p.UpdatedAt)
}
