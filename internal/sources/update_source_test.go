package sources

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestUpdateSource_ValidUpdate_SourceFieldsOnly(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create initial source
	source := SeedSource(t, db, project, datastore.APIKeyVerifier)

	// Update source fields
	source.Name = "UpdatedSourceName"
	source.IsDisabled = true
	source.ForwardHeaders = []string{"X-New-Header"}
	source.IdempotencyKeys = []string{"X-Idempotency"}

	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify updates
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, "UpdatedSourceName", fetched.Name)
	require.True(t, fetched.IsDisabled)
	require.Len(t, fetched.ForwardHeaders, 1)
	require.Contains(t, fetched.ForwardHeaders, "X-New-Header")
	require.Len(t, fetched.IdempotencyKeys, 1)
	require.Contains(t, fetched.IdempotencyKeys, "X-Idempotency")
}

func TestUpdateSource_ChangeVerifierType_APIKeyToHMac(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source with APIKey verifier
	source := SeedSource(t, db, project, datastore.APIKeyVerifier)
	originalVerifierID := source.VerifierID

	// Change to HMac verifier
	source.Verifier = &datastore.VerifierConfig{
		Type: datastore.HMacVerifier,
		HMac: &datastore.HMac{
			Hash:     "SHA512",
			Header:   "X-Signature",
			Secret:   "new-secret",
			Encoding: datastore.HexEncoding,
		},
	}

	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify verifier was updated
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.HMacVerifier, fetched.Verifier.Type)
	require.NotNil(t, fetched.Verifier.HMac)
	require.Equal(t, "SHA512", fetched.Verifier.HMac.Hash)
	require.Equal(t, "X-Signature", fetched.Verifier.HMac.Header)
	require.Equal(t, "new-secret", fetched.Verifier.HMac.Secret)
	require.Equal(t, datastore.HexEncoding, fetched.Verifier.HMac.Encoding)
	require.Equal(t, originalVerifierID, fetched.VerifierID) // ID should remain same
}

func TestUpdateSource_ChangeVerifierType_BasicAuthToAPIKey(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source with BasicAuth verifier
	source := SeedSource(t, db, project, datastore.BasicAuthVerifier)

	// Change to APIKey verifier
	source.Verifier = &datastore.VerifierConfig{
		Type: datastore.APIKeyVerifier,
		ApiKey: &datastore.ApiKey{
			HeaderName:  "Authorization",
			HeaderValue: "Bearer token123",
		},
	}

	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify verifier was updated
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.APIKeyVerifier, fetched.Verifier.Type)
	require.NotNil(t, fetched.Verifier.ApiKey)
	require.Equal(t, "Authorization", fetched.Verifier.ApiKey.HeaderName)
	require.Equal(t, "Bearer token123", fetched.Verifier.ApiKey.HeaderValue)
}

func TestUpdateSource_UpdateVerifierCredentials(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source with APIKey verifier
	source := SeedSource(t, db, project, datastore.APIKeyVerifier)

	// Update only the API key value
	source.Verifier.ApiKey.HeaderValue = "new-api-key-value"

	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify credentials were updated
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.APIKeyVerifier, fetched.Verifier.Type)
	require.Equal(t, "new-api-key-value", fetched.Verifier.ApiKey.HeaderValue)
}

func TestUpdateSource_UpdateCustomResponse(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Add custom response
	source.CustomResponse = datastore.CustomResponse{
		Body:        "Accepted",
		ContentType: "application/json",
	}

	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify custom response
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, "Accepted", fetched.CustomResponse.Body)
	require.Equal(t, "application/json", fetched.CustomResponse.ContentType)
}

func TestUpdateSource_UpdatePubSubConfig(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create PubSub source
	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "PubSubSource",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.GooglePubSub,
			Workers: 5,
			Google: &datastore.GooglePubSubConfig{
				ProjectID:      "original-project",
				SubscriptionID: "original-subscription",
			},
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Update PubSub config
	source.PubSub.Workers = 10
	source.PubSub.Google.ProjectID = "updated-project"
	source.PubSub.Google.SubscriptionID = "updated-subscription"

	err = service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify PubSub config update
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.PubSub)
	require.Equal(t, 10, fetched.PubSub.Workers)
	require.Equal(t, "updated-project", fetched.PubSub.Google.ProjectID)
	require.Equal(t, "updated-subscription", fetched.PubSub.Google.SubscriptionID)
}

func TestUpdateSource_AddBodyAndHeaderFunctions(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Add functions
	bodyFunc := "function transform(body) { return JSON.parse(body); }"
	headerFunc := "function addHeaders(headers) { headers['X-Custom'] = 'value'; return headers; }"
	source.BodyFunction = &bodyFunc
	source.HeaderFunction = &headerFunc

	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify functions
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.BodyFunction)
	require.Equal(t, bodyFunc, *fetched.BodyFunction)
	require.NotNil(t, fetched.HeaderFunction)
	require.Equal(t, headerFunc, *fetched.HeaderFunction)
}

func TestUpdateSource_RemoveFunctions(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	// Create source with functions
	bodyFunc := "function(body) { return body; }"
	headerFunc := "function(headers) { return headers; }"
	source := &datastore.Source{
		UID:            ulid.Make().String(),
		Name:           "TestSource",
		Type:           datastore.HTTPSource,
		MaskID:         ulid.Make().String(),
		Provider:       datastore.GithubSourceProvider,
		ProjectID:      project.UID,
		BodyFunction:   &bodyFunc,
		HeaderFunction: &headerFunc,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Remove functions
	source.BodyFunction = nil
	source.HeaderFunction = nil

	err = service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify functions removed
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Nil(t, fetched.BodyFunction)
	require.Nil(t, fetched.HeaderFunction)
}

func TestUpdateSource_SourceNotFound(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "NonExistent",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.UpdateSource(ctx, project.UID, source)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source not found")
}

func TestUpdateSource_NilSource_ShouldFail(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	err := service.UpdateSource(ctx, project.UID, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source cannot be nil")
}

func TestUpdateSource_ChangeSourceType(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := SeedSource(t, db, project, datastore.NoopVerifier)

	// Change type from HTTP to PubSub
	source.Type = datastore.PubSubSource
	source.PubSub = &datastore.PubSubConfig{
		Type:    datastore.SqsPubSub,
		Workers: 3,
		Sqs: &datastore.SQSPubSubConfig{
			QueueName: "test-queue",
		},
	}

	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify type change
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, datastore.PubSubSource, fetched.Type)
	require.NotNil(t, fetched.PubSub)
	require.Equal(t, datastore.SqsPubSub, fetched.PubSub.Type)
}

func TestUpdateSource_ToggleIsDisabled(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := SeedSource(t, db, project, datastore.NoopVerifier)
	require.False(t, source.IsDisabled)

	// Disable source
	source.IsDisabled = true
	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.True(t, fetched.IsDisabled)

	// Re-enable source
	source.IsDisabled = false
	err = service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	fetched, err = service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.False(t, fetched.IsDisabled)
}

func TestUpdateSource_UpdatedAtTimestamp(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := SeedSource(t, db, project, datastore.NoopVerifier)
	originalUpdatedAt := source.UpdatedAt

	// Update source
	source.Name = "UpdatedName"
	err := service.UpdateSource(ctx, project.UID, source)
	require.NoError(t, err)

	// Verify updated_at changed
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.True(t, fetched.UpdatedAt.After(originalUpdatedAt))
}
