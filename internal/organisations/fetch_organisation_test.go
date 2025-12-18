package organisations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestFetchOrganisationByID_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation
	org := seedOrganisation(t, db, "fetch.test.com", "fetch.convoy.io")

	// Fetch organisation
	fetched, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, org.UID, fetched.UID)
	require.Equal(t, org.Name, fetched.Name)
	require.Equal(t, org.OwnerID, fetched.OwnerID)
}

func TestFetchOrganisationByID_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to fetch non-existent organisation
	_, err := service.FetchOrganisationByID(ctx, "non-existent-id")
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgNotFound, err)
}

func TestFetchOrganisationByCustomDomain_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation with custom domain
	org := seedOrganisation(t, db, "custom.domain.com", "")

	// Fetch by custom domain
	fetched, err := service.FetchOrganisationByCustomDomain(ctx, "custom.domain.com")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, org.UID, fetched.UID)
	require.Equal(t, "custom.domain.com", fetched.CustomDomain.String)
}

func TestFetchOrganisationByCustomDomain_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to fetch by non-existent custom domain
	_, err := service.FetchOrganisationByCustomDomain(ctx, "non-existent.com")
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgNotFound, err)
}

func TestFetchOrganisationByAssignedDomain_Found(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation with assigned domain
	org := seedOrganisation(t, db, "", "assigned.convoy.io")

	// Fetch by assigned domain
	fetched, err := service.FetchOrganisationByAssignedDomain(ctx, "assigned.convoy.io")
	require.NoError(t, err)
	require.NotNil(t, fetched)
	require.Equal(t, org.UID, fetched.UID)
	require.Equal(t, "assigned.convoy.io", fetched.AssignedDomain.String)
}

func TestFetchOrganisationByAssignedDomain_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to fetch by non-existent assigned domain
	_, err := service.FetchOrganisationByAssignedDomain(ctx, "non-existent.convoy.io")
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgNotFound, err)
}

func TestFetchOrganisation_DeletedOrganisationNotReturned(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed and then delete organisation
	org := seedOrganisation(t, db, "deleted.test.com", "deleted.convoy.io")

	err := service.DeleteOrganisation(ctx, org.UID)
	require.NoError(t, err)

	// Try to fetch deleted organisation - should not be found
	_, err = service.FetchOrganisationByID(ctx, org.UID)
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgNotFound, err)

	_, err = service.FetchOrganisationByCustomDomain(ctx, "deleted.test.com")
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgNotFound, err)

	_, err = service.FetchOrganisationByAssignedDomain(ctx, "deleted.convoy.io")
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgNotFound, err)
}
