package projects

import (
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/subscriptions"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestDeleteProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)

	tests := []struct {
		name    string
		setup   func() string
		wantErr bool
	}{
		{
			name: "should_delete_project_successfully",
			setup: func() string {
				project := seedProject(t, db, org)
				return project.UID
			},
			wantErr: false,
		},
		{
			name: "should_succeed_with_non_existent_project",
			setup: func() string {
				return ulid.Make().String()
			},
			wantErr: false, // Soft delete doesn't fail if project doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectID := tt.setup()

			err := service.DeleteProject(ctx, projectID)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify project is soft deleted
				_, err := service.FetchProjectByID(ctx, projectID)
				require.Error(t, err)
				require.ErrorIs(t, err, datastore.ErrProjectNotFound)
			}
		})
	}
}

func TestDeleteProject_CascadeDeletes(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)

	// Create related entities
	endpoint := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
	event := seedEvent(t, db, project, endpoint)
	subscription := seedSubscription(t, db, project, endpoint)

	// Delete project
	err := service.DeleteProject(ctx, project.UID)
	require.NoError(t, err)

	// Verify project is deleted
	_, err = service.FetchProjectByID(ctx, project.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrProjectNotFound)

	// Verify endpoints are soft deleted
	endpointRepo := postgres.NewEndpointRepo(db)
	fetchedEndpoint, err := endpointRepo.FindEndpointByID(ctx, endpoint.UID, project.UID)
	require.Error(t, err)
	require.Nil(t, fetchedEndpoint)

	// Verify events are soft deleted
	eventRepo := postgres.NewEventRepo(db)
	fetchedEvent, err := eventRepo.FindEventByID(ctx, project.UID, event.UID)
	require.Error(t, err)
	require.Nil(t, fetchedEvent)

	// Verify subscriptions are soft deleted
	subRepo := subscriptions.New(log.NewLogger(os.Stdout), db)
	fetchedSub, err := subRepo.FindSubscriptionByID(ctx, project.UID, subscription.UID)
	require.Error(t, err)
	require.Nil(t, fetchedSub)
}

func TestDeleteProject_WithMultipleEndpoints(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)

	// Create multiple endpoints
	endpoint1 := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
	endpoint2 := seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
	endpoint3 := seedEndpoint(t, db, project, datastore.InactiveEndpointStatus)

	// Delete project
	err := service.DeleteProject(ctx, project.UID)
	require.NoError(t, err)

	// Verify all endpoints are deleted
	endpointRepo := postgres.NewEndpointRepo(db)

	_, err = endpointRepo.FindEndpointByID(ctx, endpoint1.UID, project.UID)
	require.Error(t, err)

	_, err = endpointRepo.FindEndpointByID(ctx, endpoint2.UID, project.UID)
	require.Error(t, err)

	_, err = endpointRepo.FindEndpointByID(ctx, endpoint3.UID, project.UID)
	require.Error(t, err)
}

func TestDeleteProject_TransactionIntegrity(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(testLogger, db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)

	// Create related entities
	_ = seedEndpoint(t, db, project, datastore.ActiveEndpointStatus)
	_ = seedEvent(t, db, project, nil)

	// Delete project
	err := service.DeleteProject(ctx, project.UID)
	require.NoError(t, err)

	// Verify everything was deleted in the transaction
	// If transaction rolled back, project would still exist
	_, err = service.FetchProjectByID(ctx, project.UID)
	require.Error(t, err)
	require.ErrorIs(t, err, datastore.ErrProjectNotFound)
}
