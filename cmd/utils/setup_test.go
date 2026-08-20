package utils

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/broker"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
)

var (
	infra *testenv.Environment
)

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to launch test infrastructure: %v\n", err)
		os.Exit(1)
	}

	infra = res

	code := m.Run()

	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup test infrastructure: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}

type testInstance struct {
	Logger   log.Logger
	Conn     *pgxpool.Pool
	Redis    redis.UniversalClient
	Database database.Database
}

func newInfra(t *testing.T) (context.Context, *testInstance) {
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

	rd, err := infra.NewRedisClient(t, 0)
	require.NoError(t, err)

	return ctx, &testInstance{
		Database: pg,
		Redis:    rd,
		Conn:     conn,
		Logger:   logger,
	}
}

func newCLIApp(t *testing.T, inst *testInstance) *cli.App {
	t.Helper()

	cfg, err := config.Get()
	require.NoError(t, err)

	brokerDeps := broker.NewTest(t, cfg, inst.Database.GetDB(), inst.Logger, inst.Conn, inst.Redis)

	return &cli.App{
		DB:     inst.Database,
		Redis:  inst.Redis,
		Logger: inst.Logger,
		Cache:  brokerDeps.Cache,
		Broker: brokerDeps,
	}
}
