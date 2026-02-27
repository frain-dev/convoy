package event_types

import (
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestService_CheckEventTypeExists(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create an event type
	eventType := seedEventType(t, db, project, "exists.test", "Test", "test", []byte(`{}`))

	// Check it exists
	exists, err := service.CheckEventTypeExists(ctx, eventType.Name, project.UID)
	require.NoError(t, err)
	require.True(t, exists)

	// Check non-existent event type
	exists, err = service.CheckEventTypeExists(ctx, "non.existent", project.UID)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestService_CheckEventTypeExists_NotExists(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Check non-existent event type
	exists, err := service.CheckEventTypeExists(ctx, "does.not.exist", project.UID)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestService_CheckEventTypeExists_DifferentProjects(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, org, project1 := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create a second project
	project2 := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           "Test Project 2",
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &datastore.DefaultProjectConfig,
	}
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	err := projectRepo.CreateProject(ctx, project2)
	require.NoError(t, err)

	// Create event type in project1
	eventType := seedEventType(t, db, project1, "same.name", "Test", "test", []byte(`{}`))

	// Check it exists in project1
	exists, err := service.CheckEventTypeExists(ctx, eventType.Name, project1.UID)
	require.NoError(t, err)
	require.True(t, exists)

	// Check it does not exist in project2 (same name, different project)
	exists, err = service.CheckEventTypeExists(ctx, eventType.Name, project2.UID)
	require.NoError(t, err)
	require.False(t, exists, "Event type with same name should not exist in different project")
}
