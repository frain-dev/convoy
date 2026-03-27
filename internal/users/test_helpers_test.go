package users

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

// Test environment
var testEnv *testenv.Environment

// TestMain initializes the test environment
func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	testEnv = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}

// setupTestDB initializes a test database connection
func setupTestDB(t *testing.T) (context.Context, *Service) {
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

	logger := log.New("convoy", log.LevelInfo)
	service := New(logger, db)

	return ctx, service
}

// createTestUser creates a user for testing
func createTestUser(t *testing.T, service *Service, ctx context.Context) *datastore.User {
	t.Helper()

	uid := ulid.Make().String()
	user := &datastore.User{
		UID:           uid,
		FirstName:     "Test",
		LastName:      "User",
		Email:         fmt.Sprintf("test-%s@example.com", uid),
		Password:      "hashedpassword123",
		EmailVerified: false,
		AuthType:      "local",
	}

	err := service.CreateUser(ctx, user)
	require.NoError(t, err)

	return user
}

// assertUserEqual verifies two users have the same core fields
func assertUserEqual(t *testing.T, expected, actual *datastore.User) {
	t.Helper()

	require.Equal(t, expected.UID, actual.UID)
	require.Equal(t, expected.FirstName, actual.FirstName)
	require.Equal(t, expected.LastName, actual.LastName)
	require.Equal(t, expected.Email, actual.Email)
	require.Equal(t, expected.Password, actual.Password)
	require.Equal(t, expected.EmailVerified, actual.EmailVerified)
	require.Equal(t, expected.AuthType, actual.AuthType)
}
