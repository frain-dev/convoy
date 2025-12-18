package organisation_invites

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

// ============================================================================
// Update Tests
// ============================================================================

func TestUpdateOrganisationInvite_ChangeStatus(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Update status to accepted
	invite.Status = datastore.InviteStatusAccepted
	err := service.UpdateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify update
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.InviteStatusAccepted, fetched.Status)
}

func TestUpdateOrganisationInvite_ChangeRole(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Update role
	invite.Role = auth.Role{
		Type:    auth.RoleProjectAdmin,
		Project: project.UID,
	}
	err := service.UpdateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify update
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, auth.RoleProjectAdmin, fetched.Role.Type)
	require.Equal(t, project.UID, fetched.Role.Project)
}

func TestUpdateOrganisationInvite_ChangeExpiresAt(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Update expires_at
	newExpiry := time.Now().Add(48 * time.Hour)
	invite.ExpiresAt = newExpiry
	err := service.UpdateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify update
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.WithinDuration(t, newExpiry, fetched.ExpiresAt, time.Second)
}

func TestUpdateOrganisationInvite_AllStatuses(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	transitions := []struct {
		from datastore.InviteStatus
		to   datastore.InviteStatus
	}{
		{datastore.InviteStatusPending, datastore.InviteStatusAccepted},
		{datastore.InviteStatusPending, datastore.InviteStatusDeclined},
		{datastore.InviteStatusPending, datastore.InviteStatusCancelled},
		{datastore.InviteStatusAccepted, datastore.InviteStatusCancelled},
	}

	for _, tt := range transitions {
		t.Run(string(tt.from)+"_to_"+string(tt.to), func(t *testing.T) {
			invite := seedOrganisationInvite(t, db, org, tt.from)

			invite.Status = tt.to
			err := service.UpdateOrganisationInvite(ctx, invite)
			require.NoError(t, err)

			fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
			require.NoError(t, err)
			require.Equal(t, tt.to, fetched.Status)
		})
	}
}

func TestUpdateOrganisationInvite_UpdateMultipleFields(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	project := seedProject(t, db, org)
	endpoint := seedEndpoint(t, db, project)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Update multiple fields
	newExpiry := time.Now().Add(72 * time.Hour)

	invite.Status = datastore.InviteStatusAccepted
	invite.Role = auth.Role{
		Type:     auth.RoleProjectAdmin,
		Project:  project.UID,
		Endpoint: endpoint.UID,
	}
	invite.ExpiresAt = newExpiry

	err := service.UpdateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify all updates
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.InviteStatusAccepted, fetched.Status)
	require.Equal(t, auth.RoleProjectAdmin, fetched.Role.Type)
	require.Equal(t, project.UID, fetched.Role.Project)
	require.Equal(t, endpoint.UID, fetched.Role.Endpoint)
	require.WithinDuration(t, newExpiry, fetched.ExpiresAt, time.Second)
}

func TestUpdateOrganisationInvite_NilInvite(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	err := service.UpdateOrganisationInvite(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "organisation invite cannot be nil")
}

func TestUpdateOrganisationInvite_SoftDelete(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Soft delete via update
	invite.DeletedAt = null.TimeFrom(time.Now())
	err := service.UpdateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify invite is no longer fetchable
	_, err = service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)
}

func TestUpdateOrganisationInvite_UpdatedAtChanges(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Get initial updated_at
	initial, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	initialUpdatedAt := initial.UpdatedAt

	// Wait a bit and update
	time.Sleep(100 * time.Millisecond)
	invite.Status = datastore.InviteStatusAccepted
	err = service.UpdateOrganisationInvite(ctx, invite)
	require.NoError(t, err)

	// Verify updated_at changed
	updated, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.True(t, updated.UpdatedAt.After(initialUpdatedAt))
}

// ============================================================================
// Delete Tests
// ============================================================================

func TestDeleteOrganisationInvite_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Delete invite
	err := service.DeleteOrganisationInvite(ctx, invite.UID)
	require.NoError(t, err)

	// Verify invite is deleted (not fetchable)
	_, err = service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)
}

func TestDeleteOrganisationInvite_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to delete non-existent invite - should not error (idempotent)
	err := service.DeleteOrganisationInvite(ctx, "non-existent-id")
	require.NoError(t, err)
}

func TestDeleteOrganisationInvite_AlreadyDeleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Delete once
	err := service.DeleteOrganisationInvite(ctx, invite.UID)
	require.NoError(t, err)

	// Delete again - should not error (idempotent)
	err = service.DeleteOrganisationInvite(ctx, invite.UID)
	require.NoError(t, err)
}

func TestDeleteOrganisationInvite_DifferentStatuses(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	statuses := []datastore.InviteStatus{
		datastore.InviteStatusPending,
		datastore.InviteStatusAccepted,
		datastore.InviteStatusDeclined,
		datastore.InviteStatusCancelled,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			invite := seedOrganisationInvite(t, db, org, status)

			err := service.DeleteOrganisationInvite(ctx, invite.UID)
			require.NoError(t, err)

			_, err = service.FetchOrganisationInviteByID(ctx, invite.UID)
			require.Error(t, err)
			require.Equal(t, datastore.ErrOrgInviteNotFound, err)
		})
	}
}

func TestDeleteOrganisationInvite_DoesNotAffectOtherInvites(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create multiple invites
	invite1 := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	invite2 := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	invite3 := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Delete only invite2
	err := service.DeleteOrganisationInvite(ctx, invite2.UID)
	require.NoError(t, err)

	// Verify invite2 is deleted
	_, err = service.FetchOrganisationInviteByID(ctx, invite2.UID)
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)

	// Verify invite1 and invite3 are still fetchable
	fetched1, err := service.FetchOrganisationInviteByID(ctx, invite1.UID)
	require.NoError(t, err)
	require.Equal(t, invite1.UID, fetched1.UID)

	fetched3, err := service.FetchOrganisationInviteByID(ctx, invite3.UID)
	require.NoError(t, err)
	require.Equal(t, invite3.UID, fetched3.UID)
}

func TestDeleteOrganisationInvite_IsSoftDelete(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Delete invite
	err := service.DeleteOrganisationInvite(ctx, invite.UID)
	require.NoError(t, err)

	// Query database directly to verify it's a soft delete
	var deletedAt *time.Time
	query := "SELECT deleted_at FROM convoy.organisation_invites WHERE id = $1"
	err = db.GetDB().QueryRowContext(ctx, query, invite.UID).Scan(&deletedAt)
	require.NoError(t, err)
	require.NotNil(t, deletedAt)
	require.False(t, deletedAt.IsZero())
}
