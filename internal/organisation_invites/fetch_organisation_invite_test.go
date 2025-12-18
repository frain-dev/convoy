package organisation_invites

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestFetchOrganisationInviteByID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation and invite
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Fetch invite
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, invite.UID, fetched.UID)
	require.Equal(t, invite.OrganisationID, fetched.OrganisationID)
	require.Equal(t, invite.InviteeEmail, fetched.InviteeEmail)
	require.Equal(t, invite.Token, fetched.Token)
	require.Equal(t, invite.Status, fetched.Status)
}

func TestFetchOrganisationInviteByID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to fetch non-existent invite
	_, err := service.FetchOrganisationInviteByID(ctx, "non-existent-id")
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)
}

func TestFetchOrganisationInviteByToken_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation and invite
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Fetch by token
	fetched, err := service.FetchOrganisationInviteByToken(ctx, invite.Token)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, invite.UID, fetched.UID)
	require.Equal(t, invite.Token, fetched.Token)
	require.Equal(t, invite.InviteeEmail, fetched.InviteeEmail)
}

func TestFetchOrganisationInviteByToken_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to fetch by non-existent token
	_, err := service.FetchOrganisationInviteByToken(ctx, "non-existent-token")
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)
}

func TestFetchOrganisationInvite_VerifyAllFields(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation and invite
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Fetch and verify all fields
	fetched, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)

	require.Equal(t, invite.UID, fetched.UID)
	require.Equal(t, invite.OrganisationID, fetched.OrganisationID)
	require.Equal(t, invite.InviteeEmail, fetched.InviteeEmail)
	require.Equal(t, invite.Token, fetched.Token)
	require.Equal(t, invite.Status, fetched.Status)
	require.Equal(t, invite.Role.Type, fetched.Role.Type)
	require.Equal(t, invite.Role.Project, fetched.Role.Project)
	require.Equal(t, invite.Role.Endpoint, fetched.Role.Endpoint)
	require.NotZero(t, fetched.CreatedAt)
	require.NotZero(t, fetched.UpdatedAt)
	require.NotZero(t, fetched.ExpiresAt)
}

func TestFetchOrganisationInvite_DeletedInviteNotReturned(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed, then delete invite
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	err := service.DeleteOrganisationInvite(ctx, invite.UID)
	require.NoError(t, err)

	// Try to fetch deleted invite - should not be found
	_, err = service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)

	_, err = service.FetchOrganisationInviteByToken(ctx, invite.Token)
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgInviteNotFound, err)
}

func TestFetchOrganisationInvite_DifferentStatuses(t *testing.T) {
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

			// Fetch by ID
			fetchedByID, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
			require.NoError(t, err)
			require.Equal(t, status, fetchedByID.Status)

			// Fetch by token
			fetchedByToken, err := service.FetchOrganisationInviteByToken(ctx, invite.Token)
			require.NoError(t, err)
			require.Equal(t, status, fetchedByToken.Status)
		})
	}
}

func TestFetchOrganisationInvite_MultipleInvitesForSameOrganisation(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)

	// Create multiple invites for the same organisation
	invite1 := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	invite2 := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)
	invite3 := seedOrganisationInvite(t, db, org, datastore.InviteStatusAccepted)

	// Fetch each invite independently
	fetched1, err := service.FetchOrganisationInviteByID(ctx, invite1.UID)
	require.NoError(t, err)
	require.Equal(t, invite1.UID, fetched1.UID)
	require.Equal(t, invite1.InviteeEmail, fetched1.InviteeEmail)

	fetched2, err := service.FetchOrganisationInviteByID(ctx, invite2.UID)
	require.NoError(t, err)
	require.Equal(t, invite2.UID, fetched2.UID)
	require.Equal(t, invite2.InviteeEmail, fetched2.InviteeEmail)

	fetched3, err := service.FetchOrganisationInviteByID(ctx, invite3.UID)
	require.NoError(t, err)
	require.Equal(t, invite3.UID, fetched3.UID)
	require.Equal(t, datastore.InviteStatusAccepted, fetched3.Status)
}

func TestFetchOrganisationInvite_ByTokenReturnsSameAsById(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)
	org := seedOrganisation(t, db)
	invite := seedOrganisationInvite(t, db, org, datastore.InviteStatusPending)

	// Fetch by ID and token
	fetchedByID, err := service.FetchOrganisationInviteByID(ctx, invite.UID)
	require.NoError(t, err)

	fetchedByToken, err := service.FetchOrganisationInviteByToken(ctx, invite.Token)
	require.NoError(t, err)

	// Both should return the same invite
	require.Equal(t, fetchedByID.UID, fetchedByToken.UID)
	require.Equal(t, fetchedByID.OrganisationID, fetchedByToken.OrganisationID)
	require.Equal(t, fetchedByID.InviteeEmail, fetchedByToken.InviteeEmail)
	require.Equal(t, fetchedByID.Token, fetchedByToken.Token)
	require.Equal(t, fetchedByID.Status, fetchedByToken.Status)
	require.Equal(t, fetchedByID.Role, fetchedByToken.Role)
}
