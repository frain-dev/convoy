package loader

import (
	"context"
	"os"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var (
	infra *testenv.Environment
)

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		log.Fatalf("Failed to launch test infrastructure: %v", err)
		os.Exit(1)
	}

	infra = res

	code := m.Run()

	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
		os.Exit(1)
	}

	os.Exit(code)
}

type testInstance struct {
	Logger     *log.Logger
	Conn       *pgxpool.Pool
	KeyManager keys.KeyManager
	Database   database.Database
}

func newLoader(t *testing.T) (context.Context, *testInstance) {
	t.Helper()

	ctx := t.Context()

	logger := testenv.NewLogger(t)

	err := config.LoadConfig("")
	require.NoError(t, err)

	conn, err := infra.CloneTestDatabase(t, "convoy")
	require.NoError(t, err)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

	pg := postgres.NewFromConnection(conn)

	km, err := keys.NewLocalKeyManager()
	if err != nil {
		log.Fatal(err)
	}

	if err = keys.Set(km); err != nil {
		log.Fatal(err)
	}

	return ctx, &testInstance{
		Database:   pg,
		KeyManager: km,
		Conn:       conn,
		Logger:     logger,
	}
}
