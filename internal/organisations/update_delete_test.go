package organisations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
)

func TestUpdateOrganisation_ValidUpdate(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation
	org := seedOrganisation(t, db, "old.test.com", "old.convoy.io")

	// Update organisation
	org.Name = "Updated Organisation Name"
	org.CustomDomain = null.StringFrom("new.test.com")
	org.AssignedDomain = null.StringFrom("new.convoy.io")

	err := service.UpdateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify updates
	fetched, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.Equal(t, "Updated Organisation Name", fetched.Name)
	require.Equal(t, "new.test.com", fetched.CustomDomain.String)
	require.Equal(t, "new.convoy.io", fetched.AssignedDomain.String)
}

func TestUpdateOrganisation_UpdateName(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation
	org := seedOrganisation(t, db, "", "")

	// Update only name
	org.Name = "New Name"

	err := service.UpdateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify name was updated
	fetched, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.Equal(t, "New Name", fetched.Name)
}

func TestUpdateOrganisation_ClearCustomDomain(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation with custom domain
	org := seedOrganisation(t, db, "clear.test.com", "")

	// Clear custom domain
	org.CustomDomain = null.NewString("", false)

	err := service.UpdateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify custom domain was cleared
	fetched, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.False(t, fetched.CustomDomain.Valid)
}

func TestUpdateOrganisation_NilOrganisation(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	err := service.UpdateOrganisation(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "organisation cannot be nil")
}

func TestUpdateOrganisation_VerifyTimestampUpdated(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation
	org := seedOrganisation(t, db, "", "")

	// Get original timestamps
	original, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)

	// Update organisation
	org.Name = "Updated Name"
	err = service.UpdateOrganisation(ctx, org)
	require.NoError(t, err)

	// Verify updated_at changed
	updated, err := service.FetchOrganisationByID(ctx, org.UID)
	require.NoError(t, err)
	require.True(t, updated.UpdatedAt.After(original.UpdatedAt))
}

func TestDeleteOrganisation_Success(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation
	org := seedOrganisation(t, db, "", "")

	// Delete organisation
	err := service.DeleteOrganisation(ctx, org.UID)
	require.NoError(t, err)

	// Verify organisation cannot be fetched
	_, err = service.FetchOrganisationByID(ctx, org.UID)
	require.Error(t, err)
	require.Equal(t, datastore.ErrOrgNotFound, err)
}

func TestDeleteOrganisation_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Try to delete non-existent organisation
	err := service.DeleteOrganisation(ctx, "non-existent-id")
	require.Error(t, err)
	require.Contains(t, err.Error(), "organisation not found")
}

func TestDeleteOrganisation_IdempotentDelete(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Seed organisation
	org := seedOrganisation(t, db, "", "")

	// Delete organisation
	err := service.DeleteOrganisation(ctx, org.UID)
	require.NoError(t, err)

	// Try to delete again - should return not found
	err = service.DeleteOrganisation(ctx, org.UID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "organisation not found")
}

func TestCountOrganisations(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := New(log.NewLogger(os.Stdout), db)

	// Get initial count
	initialCount, err := service.CountOrganisations(ctx)
	require.NoError(t, err)

	// Seed 3 organisations
	seedOrganisation(t, db, "", "")
	seedOrganisation(t, db, "", "")
	seedOrganisation(t, db, "", "")

	// Get new count
	newCount, err := service.CountOrganisations(ctx)
	require.NoError(t, err)
	require.Equal(t, initialCount+3, newCount)

	// Delete one
	org := seedOrganisation(t, db, "", "")
	err = service.DeleteOrganisation(ctx, org.UID)
	require.NoError(t, err)

	// Count should not include deleted
	finalCount, err := service.CountOrganisations(ctx)
	require.NoError(t, err)
	require.Equal(t, newCount, finalCount) // Same as before because we deleted the one we just added
}
