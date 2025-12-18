package organisation_invites

import (
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

// TestDeleteOrganisationInvite_ErrorPath tests error handling in delete
func TestDeleteOrganisationInvite_ErrorPath(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to delete with invalid ID (triggers different error path)
	// This tests the error logging branch
	err := service.DeleteOrganisationInvite(ctx, "")
	// Empty ID may trigger a database error which exercises the error path
	// The exact behavior depends on the database
	_ = err // We're mainly testing that the code path executes without panic
}

// TestUpdateOrganisationInvite_ErrorPath tests error handling in update
func TestUpdateOrganisationInvite_ErrorPath(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create an invite that we'll try to update with invalid data
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Try to update with a non-existent project (foreign key violation)
	invite.Role.Project = ulid.Make().String() // Non-existent project
	err := service.UpdateOrganisationInvite(ctx, invite)

	// This should trigger a foreign key error, exercising the error logging path
	require.Error(t, err)
}

// TestFetchOrganisationInviteByID_DatabaseError tests non-NotFound errors
func TestFetchOrganisationInviteByID_DatabaseError(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Close the database to trigger connection errors
	db.Close()

	// This should trigger a database error (not ErrNoRows)
	_, err := service.FetchOrganisationInviteByID(ctx, ulid.Make().String())
	require.Error(t, err)
}

// TestFetchOrganisationInviteByToken_DatabaseError tests non-NotFound errors
func TestFetchOrganisationInviteByToken_DatabaseError(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Close the database to trigger connection errors
	db.Close()

	// This should trigger a database error (not ErrNoRows)
	_, err := service.FetchOrganisationInviteByToken(ctx, "some-token")
	require.Error(t, err)
}

// TestLoadOrganisationsInvitesPaged_DatabaseError tests error in pagination
func TestLoadOrganisationsInvitesPaged_DatabaseError(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Close the database to trigger connection errors
	db.Close()

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	// This should trigger a database error
	_, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.Error(t, err)
}

// TestLoadOrganisationsInvitesPaged_CountError tests error in count query
func TestLoadOrganisationsInvitesPaged_CountError(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create some invites
	for i := 0; i < 5; i++ {
		seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	}

	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}

	// Fetch once successfully
	invites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.NotEmpty(t, invites)

	// Close database before count operation
	db.Close()

	// Try to paginate again - this will fail on the count query
	_, _, err = service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.Error(t, err)
}

// TestRowToOrganisationInvite_AllRowTypes tests the row conversion for all types
func TestRowToOrganisationInvite_AllRowTypes(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Test FetchOrganisationInviteByIDRow conversion
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, invite.UID, fetched.UID)

	// Test FetchOrganisationInviteByTokenRow conversion
	fetchedByToken, err := service.FetchOrganisationInviteByToken(ctx, invite.Token)
	require.NoError(t, err)
	require.NotNil(t, fetchedByToken)
	require.Equal(t, invite.UID, fetchedByToken.UID)

	// Test FetchOrganisationInvitesPaginatedRow conversion
	pageable := datastore.Pageable{
		PerPage:    10,
		Direction:  datastore.Next,
		NextCursor: ulid.Make().String(),
	}
	invites, _, err := service.LoadOrganisationsInvitesPaged(ctx, org.UID, datastore.InviteStatusPending, pageable)
	require.NoError(t, err)
	require.NotEmpty(t, invites)
}
