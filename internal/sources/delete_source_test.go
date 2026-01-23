package sources

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
)

func TestDeleteSource_WithVerifier(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source with verifier
	source := SeedSource(t, db, project, datastore.APIKeyVerifier)

	// Delete source
	err := service.DeleteSourceByID(ctx, project.UID, source.UID, source.VerifierID)
	require.NoError(t, err)

	// Verify source is soft deleted
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.Error(t, err)
	require.Nil(t, fetched)
	require.Equal(t, datastore.ErrSourceNotFound, err)
}

func TestDeleteSource_WithoutVerifier(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source without verifier
	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Delete source
	err := service.DeleteSourceByID(ctx, project.UID, source.UID, "")
	require.NoError(t, err)

	// Verify source is soft deleted
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.Error(t, err)
	require.Nil(t, fetched)
	require.Equal(t, datastore.ErrSourceNotFound, err)
}

func TestDeleteSource_CascadeToSubscriptions(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source
	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Create endpoint for subscription
	endpointRepo := postgres.NewEndpointRepo(db)
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "TestEndpoint",
		Status:    datastore.ActiveEndpointStatus,
		Url:       "https://example.com/webhook",
		Secrets: []datastore.Secret{
			{Value: "secret123"},
		},
	}
	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Create subscription
	subRepo := postgres.NewSubscriptionRepo(db)
	subscription := &datastore.Subscription{
		UID:        ulid.Make().String(),
		Name:       "TestSubscription",
		Type:       datastore.SubscriptionTypeAPI,
		ProjectID:  project.UID,
		SourceID:   source.UID,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
		},
	}
	err = subRepo.CreateSubscription(ctx, project.UID, subscription)
	require.NoError(t, err)

	// Verify subscription exists
	fetchedSub, err := subRepo.FindSubscriptionByID(ctx, project.UID, subscription.UID)
	require.NoError(t, err)
	require.NotNil(t, fetchedSub)

	// Delete source (should cascade to subscription)
	err = service.DeleteSourceByID(ctx, project.UID, source.UID, source.VerifierID)
	require.NoError(t, err)

	// Verify subscription is soft deleted
	fetchedSub, err = subRepo.FindSubscriptionByID(ctx, project.UID, subscription.UID)
	require.Error(t, err)
	require.Nil(t, fetchedSub)
}

func TestDeleteSource_NotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	nonExistentID := ulid.Make().String()

	// Try to delete non-existent source
	err := service.DeleteSourceByID(ctx, project.UID, nonExistentID, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "source not found")
}

func TestDeleteSource_WrongProject(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project1 := seedTestData(t, db)
	project2 := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source in project1
	source := SeedSource(t, db, project1, datastore.NoopVerifier)

	// Try to delete using project2 ID
	err := service.DeleteSourceByID(ctx, project2.UID, source.UID, source.VerifierID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source not found")

	// Verify source still exists in project1
	fetched, err := service.FindSourceByID(ctx, project1.UID, source.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched)
}

func TestDeleteSource_AlreadyDeleted(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Delete once
	err := service.DeleteSourceByID(ctx, project.UID, source.UID, source.VerifierID)
	require.NoError(t, err)

	// Try to delete again
	err = service.DeleteSourceByID(ctx, project.UID, source.UID, source.VerifierID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source not found")
}

func TestDeleteSource_MultipleSubscriptions(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source
	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Create endpoint
	endpointRepo := postgres.NewEndpointRepo(db)
	endpoint := &datastore.Endpoint{
		UID:       ulid.Make().String(),
		ProjectID: project.UID,
		Name:      "TestEndpoint",
		Status:    datastore.ActiveEndpointStatus,
		Url:       "https://example.com/webhook",
		Secrets: []datastore.Secret{
			{Value: "secret123"},
		},
	}
	err := endpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
	require.NoError(t, err)

	// Create multiple subscriptions
	subRepo := postgres.NewSubscriptionRepo(db)
	sub1 := &datastore.Subscription{
		UID:        ulid.Make().String(),
		Name:       "Subscription1",
		Type:       datastore.SubscriptionTypeAPI,
		ProjectID:  project.UID,
		SourceID:   source.UID,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
		},
	}
	err = subRepo.CreateSubscription(ctx, project.UID, sub1)
	require.NoError(t, err)

	sub2 := &datastore.Subscription{
		UID:        ulid.Make().String(),
		Name:       "Subscription2",
		Type:       datastore.SubscriptionTypeAPI,
		ProjectID:  project.UID,
		SourceID:   source.UID,
		EndpointID: endpoint.UID,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{"*"},
		},
	}
	err = subRepo.CreateSubscription(ctx, project.UID, sub2)
	require.NoError(t, err)

	// Delete source
	err = service.DeleteSourceByID(ctx, project.UID, source.UID, source.VerifierID)
	require.NoError(t, err)

	// Verify all subscriptions are soft deleted
	fetchedSub1, err := subRepo.FindSubscriptionByID(ctx, project.UID, sub1.UID)
	require.Error(t, err)
	require.Nil(t, fetchedSub1)

	fetchedSub2, err := subRepo.FindSubscriptionByID(ctx, project.UID, sub2.UID)
	require.Error(t, err)
	require.Nil(t, fetchedSub2)
}

func TestDeleteSource_DifferentVerifierTypes(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	testCases := []struct {
		name         string
		verifierType datastore.VerifierType
	}{
		{"APIKey", datastore.APIKeyVerifier},
		{"BasicAuth", datastore.BasicAuthVerifier},
		{"HMac", datastore.HMacVerifier},
		{"Noop", datastore.NoopVerifier},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			source := SeedSource(t, db, project, tc.verifierType)

			err := service.DeleteSourceByID(ctx, project.UID, source.UID, source.VerifierID)
			require.NoError(t, err)

			// Verify deletion
			fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
			require.Error(t, err)
			require.Nil(t, fetched)
		})
	}
}

func TestDeleteSource_PreservesOtherSources(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create multiple sources
	source1 := SeedSource(t, db, project, datastore.NoopVerifier)
	source2 := SeedSource(t, db, project, datastore.APIKeyVerifier)
	source3 := SeedSource(t, db, project, datastore.BasicAuthVerifier)

	// Delete only source2
	err := service.DeleteSourceByID(ctx, project.UID, source2.UID, source2.VerifierID)
	require.NoError(t, err)

	// Verify source2 is deleted
	fetched2, err := service.FindSourceByID(ctx, project.UID, source2.UID)
	require.Error(t, err)
	require.Nil(t, fetched2)

	// Verify source1 and source3 still exist
	fetched1, err := service.FindSourceByID(ctx, project.UID, source1.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched1)

	fetched3, err := service.FindSourceByID(ctx, project.UID, source3.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched3)
}

func TestDeleteSource_IsSoftDelete(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Delete source
	err := service.DeleteSourceByID(ctx, project.UID, source.UID, source.VerifierID)
	require.NoError(t, err)

	// Verify it's a soft delete by checking directly in DB
	// The source should have deleted_at set, not be physically removed
	var count int
	err = db.GetDB().QueryRowContext(ctx,
		"SELECT COUNT(*) FROM convoy.sources WHERE id = $1",
		source.UID,
	).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 1, count, "Source should still exist in database (soft delete)")

	// Verify deleted_at is set
	var deletedAt interface{}
	err = db.GetDB().QueryRowContext(ctx,
		"SELECT deleted_at FROM convoy.sources WHERE id = $1",
		source.UID,
	).Scan(&deletedAt)
	require.NoError(t, err)
	require.NotNil(t, deletedAt, "deleted_at should be set")
}
