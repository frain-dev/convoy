package event_types

import (
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestService_CreateEventType(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	tests := []struct {
		name        string
		eventType   *datastore.ProjectEventType
		wantErr     bool
		errContains string
	}{
		{
			name: "valid request with all fields",
			eventType: &datastore.ProjectEventType{
				UID:         ulid.Make().String(),
				Name:        "user.created",
				ProjectId:   project.UID,
				Description: "User account created event",
				Category:    "authentication",
				JSONSchema:  []byte(`{"type": "object", "properties": {"user_id": {"type": "string"}}}`),
			},
			wantErr: false,
		},
		{
			name: "valid request with minimal fields",
			eventType: &datastore.ProjectEventType{
				UID:        ulid.Make().String(),
				Name:       "order.placed",
				ProjectId:  project.UID,
				JSONSchema: []byte(`{}`),
			},
			wantErr: false,
		},
		{
			name: "with JSON schema",
			eventType: &datastore.ProjectEventType{
				UID:        ulid.Make().String(),
				Name:       "payment.processed",
				ProjectId:  project.UID,
				JSONSchema: []byte(`{"type": "object", "required": ["amount", "currency"]}`),
			},
			wantErr: false,
		},
		{
			name:        "nil event type should error",
			eventType:   nil,
			wantErr:     true,
			errContains: "could not be created",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.CreateEventType(ctx, tt.eventType)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					require.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)

			// Verify the event type was created
			if tt.eventType != nil {
				fetched, err := service.FetchEventTypeById(ctx, tt.eventType.UID, project.UID)
				require.NoError(t, err)
				require.Equal(t, tt.eventType.UID, fetched.UID)
				require.Equal(t, tt.eventType.Name, fetched.Name)
				require.Equal(t, tt.eventType.ProjectId, fetched.ProjectId)
				require.Equal(t, tt.eventType.Description, fetched.Description)
				require.Equal(t, tt.eventType.Category, fetched.Category)
				require.Equal(t, tt.eventType.JSONSchema, fetched.JSONSchema)

				// Verify timestamps are set
				require.False(t, fetched.CreatedAt.IsZero())
				require.False(t, fetched.UpdatedAt.IsZero())
				require.False(t, fetched.DeprecatedAt.Valid)
			}
		})
	}
}

func TestService_CreateEventType_TimestampsSet(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	before := time.Now().Add(-time.Second)

	eventType := &datastore.ProjectEventType{
		UID:        ulid.Make().String(),
		Name:       "test.event",
		ProjectId:  project.UID,
		JSONSchema: []byte(`{}`),
	}

	err := service.CreateEventType(ctx, eventType)
	require.NoError(t, err)

	after := time.Now().Add(time.Second)

	fetched, err := service.FetchEventTypeById(ctx, eventType.UID, project.UID)
	require.NoError(t, err)

	require.True(t, fetched.CreatedAt.After(before) && fetched.CreatedAt.Before(after))
	require.True(t, fetched.UpdatedAt.After(before) && fetched.UpdatedAt.Before(after))
}

func TestService_CreateDefaultEventType(t *testing.T) {
	db, ctx := setupTestDB(t)
	_, _, project := seedTestData(t, db)
	service := createEventTypeService(t, db)

	err := service.CreateDefaultEventType(ctx, project.UID)
	require.NoError(t, err)

	// Verify the default event type was created with wildcard name
	eventType, err := service.FetchEventTypeByName(ctx, "*", project.UID)
	require.NoError(t, err)
	require.Equal(t, "*", eventType.Name)
	require.Equal(t, project.UID, eventType.ProjectId)
	require.Equal(t, []byte(`{}`), eventType.JSONSchema)
	require.Empty(t, eventType.Description)
	require.Empty(t, eventType.Category)
}
