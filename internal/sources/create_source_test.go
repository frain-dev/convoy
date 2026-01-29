package sources

import (
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func TestCreateSource_ValidRequest_NoVerifier(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:        ulid.Make().String(),
		Name:       "TestSource",
		Type:       datastore.HTTPSource,
		MaskID:     ulid.Make().String(),
		Provider:   datastore.GithubSourceProvider,
		IsDisabled: false,
		ProjectID:  project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify source was created
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, source.UID, fetched.UID)
	require.Equal(t, source.Name, fetched.Name)
	require.Equal(t, source.Type, fetched.Type)
	require.Equal(t, source.Provider, fetched.Provider)
	require.Equal(t, datastore.NoopVerifier, fetched.Verifier.Type)
}

func TestCreateSource_WithAPIKeyVerifier(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "TestSource",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.APIKeyVerifier,
			ApiKey: &datastore.ApiKey{
				HeaderName:  "X-API-Key",
				HeaderValue: "test-api-key-123",
			},
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify source and verifier were created
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, source.UID, fetched.UID)
	require.Equal(t, datastore.APIKeyVerifier, fetched.Verifier.Type)
	require.NotNil(t, fetched.Verifier.ApiKey)
	require.Equal(t, "X-API-Key", fetched.Verifier.ApiKey.HeaderName)
	require.Equal(t, "test-api-key-123", fetched.Verifier.ApiKey.HeaderValue)
	require.NotEmpty(t, fetched.VerifierID)
}

func TestCreateSource_WithBasicAuthVerifier(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "TestSource",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.ShopifySourceProvider,
		ProjectID: project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.BasicAuthVerifier,
			BasicAuth: &datastore.BasicAuth{
				UserName: "testuser",
				Password: "testpassword",
			},
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify source and verifier were created
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, source.UID, fetched.UID)
	require.Equal(t, datastore.BasicAuthVerifier, fetched.Verifier.Type)
	require.NotNil(t, fetched.Verifier.BasicAuth)
	require.Equal(t, "testuser", fetched.Verifier.BasicAuth.UserName)
	require.Equal(t, "testpassword", fetched.Verifier.BasicAuth.Password)
	require.NotEmpty(t, fetched.VerifierID)
}

func TestCreateSource_WithHMacVerifier(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "TestSource",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.TwitterSourceProvider,
		ProjectID: project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &datastore.HMac{
				Hash:     "SHA256",
				Header:   "X-Webhook-Signature",
				Secret:   "my-secret-key",
				Encoding: datastore.Base64Encoding,
			},
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify source and verifier were created
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, source.UID, fetched.UID)
	require.Equal(t, datastore.HMacVerifier, fetched.Verifier.Type)
	require.NotNil(t, fetched.Verifier.HMac)
	require.Equal(t, "SHA256", fetched.Verifier.HMac.Hash)
	require.Equal(t, "X-Webhook-Signature", fetched.Verifier.HMac.Header)
	require.Equal(t, "my-secret-key", fetched.Verifier.HMac.Secret)
	require.Equal(t, datastore.Base64Encoding, fetched.Verifier.HMac.Encoding)
	require.NotEmpty(t, fetched.VerifierID)
}

func TestCreateSource_WithForwardHeaders(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:            ulid.Make().String(),
		Name:           "TestSource",
		Type:           datastore.HTTPSource,
		MaskID:         ulid.Make().String(),
		Provider:       datastore.GithubSourceProvider,
		ProjectID:      project.UID,
		ForwardHeaders: []string{"X-Custom-Header", "X-Another-Header"},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify forward headers
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Len(t, fetched.ForwardHeaders, 2)
	require.Contains(t, fetched.ForwardHeaders, "X-Custom-Header")
	require.Contains(t, fetched.ForwardHeaders, "X-Another-Header")
}

func TestCreateSource_WithCustomResponse(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "TestSource",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		CustomResponse: datastore.CustomResponse{
			Body:        "OK",
			ContentType: "text/plain",
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify custom response
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, "OK", fetched.CustomResponse.Body)
	require.Equal(t, "text/plain", fetched.CustomResponse.ContentType)
}

func TestCreateSource_WithIdempotencyKeys(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:             ulid.Make().String(),
		Name:            "TestSource",
		Type:            datastore.HTTPSource,
		MaskID:          ulid.Make().String(),
		Provider:        datastore.GithubSourceProvider,
		ProjectID:       project.UID,
		IdempotencyKeys: []string{"X-Request-Id", "X-Idempotency-Key"},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify idempotency keys
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Len(t, fetched.IdempotencyKeys, 2)
	require.Contains(t, fetched.IdempotencyKeys, "X-Request-Id")
	require.Contains(t, fetched.IdempotencyKeys, "X-Idempotency-Key")
}

func TestCreateSource_WithPubSubConfig(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "TestSource",
		Type:      datastore.PubSubSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		PubSub: &datastore.PubSubConfig{
			Type:    datastore.GooglePubSub,
			Workers: 5,
			Google: &datastore.GooglePubSubConfig{
				ProjectID:      "test-project",
				SubscriptionID: "test-subscription",
			},
		},
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Verify PubSub config
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.PubSub)
	require.Equal(t, datastore.GooglePubSub, fetched.PubSub.Type)
	require.NotNil(t, fetched.PubSub.Google)
	require.Equal(t, "test-project", fetched.PubSub.Google.ProjectID)
	require.Equal(t, "test-subscription", fetched.PubSub.Google.SubscriptionID)
}

func TestCreateSource_WithBodyAndHeaderFunctions(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

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

	// Verify functions
	fetched, err := service.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.NotNil(t, fetched.BodyFunction)
	require.Equal(t, bodyFunc, *fetched.BodyFunction)
	require.NotNil(t, fetched.HeaderFunction)
	require.Equal(t, headerFunc, *fetched.HeaderFunction)
}

func TestCreateSource_NilSource_ShouldFail(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	service := createSourceService(t, db)

	err := service.CreateSource(ctx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "source cannot be nil")
}

func TestCreateSource_VerifyDatabasePersistence(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      "TestSource",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.APIKeyVerifier,
			ApiKey: &datastore.ApiKey{
				HeaderName:  "X-API-Key",
				HeaderValue: "test-key",
			},
		},
	}

	err := service.CreateSource(ctx, source)
	require.NoError(t, err)

	// Create new service instance to ensure data is actually persisted
	newService := createSourceService(t, db)
	fetched, err := newService.FindSourceByID(ctx, project.UID, source.UID)
	require.NoError(t, err)
	require.Equal(t, source.UID, fetched.UID)
	require.Equal(t, source.Name, fetched.Name)
	require.NotEmpty(t, fetched.CreatedAt)
	require.NotEmpty(t, fetched.UpdatedAt)
}

func TestCreateSource_DuplicateID_ShouldFail(t *testing.T) {
	db, ctx := setupTestDB(t)
	defer db.Close()

	project := seedTestData(t, db)
	service := createSourceService(t, db)

	sourceID := ulid.Make().String()

	source1 := &datastore.Source{
		UID:       sourceID,
		Name:      "TestSource1",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err := service.CreateSource(ctx, source1)
	require.NoError(t, err)

	// Try to create another source with same ID
	source2 := &datastore.Source{
		UID:       sourceID,
		Name:      "TestSource2",
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
		Verifier: &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		},
	}

	err = service.CreateSource(ctx, source2)
	require.Error(t, err)
}
