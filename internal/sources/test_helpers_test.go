package sources

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/organisations"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var (
	testEnv *testenv.Environment
)

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}

func setupTestDB(t *testing.T) (database.Database, context.Context) {
	t.Helper()

	if testEnv == nil {
		t.Fatal("testEnv is nil - TestMain may not have run successfully")
	}

	ctx := context.Background()

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := testEnv.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	db := postgres.NewFromConnection(conn)

	// Load config again and ensure it's set properly
	err = config.LoadConfig("")
	require.NoError(t, err)

	_, err = config.Get()
	require.NoError(t, err)

	// Initialize KeyManager
	km, err := keys.NewLocalKeyManager("test")
	require.NoError(t, err)

	if km.IsSet() {
		_, err = km.GetCurrentKeyFromCache()
		require.NoError(t, err)
	}

	err = keys.Set(km)
	require.NoError(t, err)

	return db, ctx
}

func seedTestData(t *testing.T, db database.Database) *datastore.Project {
	t.Helper()

	logger := log.NewLogger(os.Stdout)
	ctx := context.Background()

	// Create user
	userRepo := users.New(logger, db)
	user := &datastore.User{
		UID:       ulid.Make().String(),
		FirstName: "Test",
		LastName:  "User",
		Email:     fmt.Sprintf("test-%s@example.com", ulid.Make().String()),
	}
	err := userRepo.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create organisation
	orgRepo := organisations.New(logger, db)
	org := &datastore.Organisation{
		UID:     ulid.Make().String(),
		OwnerID: user.UID,
		Name:    fmt.Sprintf("TestOrg-%s", ulid.Make().String()),
	}
	err = orgRepo.CreateOrganisation(ctx, org)
	require.NoError(t, err)

	// Create project
	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	projectConfig := datastore.DefaultProjectConfig
	project := &datastore.Project{
		UID:            ulid.Make().String(),
		Name:           fmt.Sprintf("TestProject-%s", ulid.Make().String()),
		Type:           datastore.OutgoingProject,
		OrganisationID: org.UID,
		Config:         &projectConfig,
	}
	err = projectRepo.CreateProject(ctx, project)
	require.NoError(t, err)

	return project
}

func createSourceService(t *testing.T, db database.Database) *Service {
	t.Helper()

	return New(log.NewLogger(os.Stdout), db)
}

// SeedSource creates a test source
func SeedSource(t *testing.T, db database.Database, project *datastore.Project, verifierType datastore.VerifierType) *datastore.Source {
	t.Helper()

	source := &datastore.Source{
		UID:       ulid.Make().String(),
		Name:      fmt.Sprintf("TestSource-%s", ulid.Make().String()),
		Type:      datastore.HTTPSource,
		MaskID:    ulid.Make().String(),
		Provider:  datastore.GithubSourceProvider,
		ProjectID: project.UID,
	}

	// Add verifier based on type
	switch verifierType {
	case datastore.APIKeyVerifier:
		source.Verifier = &datastore.VerifierConfig{
			Type: datastore.APIKeyVerifier,
			ApiKey: &datastore.ApiKey{
				HeaderName:  "X-API-Key",
				HeaderValue: "test-api-key",
			},
		}
	case datastore.BasicAuthVerifier:
		source.Verifier = &datastore.VerifierConfig{
			Type: datastore.BasicAuthVerifier,
			BasicAuth: &datastore.BasicAuth{
				UserName: "testuser",
				Password: "testpass",
			},
		}
	case datastore.HMacVerifier:
		source.Verifier = &datastore.VerifierConfig{
			Type: datastore.HMacVerifier,
			HMac: &datastore.HMac{
				Hash:     "SHA256",
				Header:   "X-Webhook-Signature",
				Secret:   "test-secret",
				Encoding: datastore.Base64Encoding,
			},
		}
	default:
		source.Verifier = &datastore.VerifierConfig{
			Type: datastore.NoopVerifier,
		}
	}

	service := createSourceService(t, db)
	err := service.CreateSource(context.Background(), source)
	require.NoError(t, err)

	return source
}
