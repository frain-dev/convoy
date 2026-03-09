package event_types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestService_UpdateEventType(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Seed an event type
	_ = seedEventType(t, db, project, "user.created", "Original description", "auth", []byte(`{}`))

	tests := []struct {
		name        string
		updateFn    func(*datastore.ProjectEventType)
		wantErr     bool
		errContains string
	}{
		{
			name: "update description only",
			updateFn: func(et *datastore.ProjectEventType) {
				et.Description = "Updated description"
			},
			wantErr: false,
		},
		{
			name: "update category only",
			updateFn: func(et *datastore.ProjectEventType) {
				et.Category = "user-management"
			},
			wantErr: false,
		},
		{
			name: "update JSON schema only",
			updateFn: func(et *datastore.ProjectEventType) {
				et.JSONSchema = []byte(`{"type": "object", "properties": {"id": {"type": "string"}}}`)
			},
			wantErr: false,
		},
		{
			name: "update all fields together",
			updateFn: func(et *datastore.ProjectEventType) {
				et.Description = "All fields updated"
				et.Category = "updated-category"
				et.JSONSchema = []byte(`{"updated": true}`)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh event type for this test
			testEventType := seedEventType(t, db, project, "test.event."+t.Name(), "desc", "cat", []byte(`{}`))
			testOriginalUpdatedAt := testEventType.UpdatedAt
			time.Sleep(10 * time.Millisecond)

			// Apply updates
			tt.updateFn(testEventType)

			err := service.UpdateEventType(ctx, testEventType)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)

			// Verify the event type was updated
			fetched, err := service.FetchEventTypeById(ctx, testEventType.UID, project.UID)
			require.NoError(t, err)
			require.Equal(t, testEventType.Description, fetched.Description)
			require.Equal(t, testEventType.Category, fetched.Category)
			require.Equal(t, testEventType.JSONSchema, fetched.JSONSchema)

			// Verify updated_at changed
			require.True(t, fetched.UpdatedAt.After(testOriginalUpdatedAt))
		})
	}
}

func TestService_UpdateEventType_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Try to update non-existent event type
	eventType := &datastore.ProjectEventType{
		UID:         "non-existent-id",
		Name:        "test",
		ProjectId:   project.UID,
		Description: "Updated",
		JSONSchema:  []byte(`{}`),
	}

	err := service.UpdateEventType(ctx, eventType)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not be updated")
}

func TestService_UpdateEventType_NilEventType(t *testing.T) {
	db, ctx := setupTestDB(t)
	service := createEventTypeService(t, db)

	err := service.UpdateEventType(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not be updated")
}

func TestService_UpdateEventType_VerifyUpdatedAtChanges(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	// Create event type
	eventType := seedEventType(t, db, project, "time.test", "desc", "cat", []byte(`{}`))
	originalUpdatedAt := eventType.UpdatedAt

	// Wait a bit to ensure updated_at will be different
	time.Sleep(100 * time.Millisecond)

	// Update the event type
	eventType.Description = "New description"
	err := service.UpdateEventType(ctx, eventType)
	require.NoError(t, err)

	// Fetch and verify updated_at changed
	fetched, err := service.FetchEventTypeById(ctx, eventType.UID, project.UID)
	require.NoError(t, err)
	require.True(t, fetched.UpdatedAt.After(originalUpdatedAt))
	require.NotEqual(t, originalUpdatedAt, fetched.UpdatedAt)
}
