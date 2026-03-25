package task

import (
	"context"
	"fmt"
	"log/slog"
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
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/delivery_attempts"
	"github.com/frain-dev/convoy/internal/event_deliveries"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
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
	Logger     log.Logger
	Conn       *pgxpool.Pool
	Config     config.Configuration
	KeyManager keys.KeyManager
	Database   database.Database
	Redis      redis.UniversalClient
	Context    context.Context
}

func newInfra(t *testing.T) *testInstance {
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

	err = config.LoadConfig("")
	require.NoError(t, err)

	cfg, err := config.Get()
	require.NoError(t, err)

	// Load CA cert for TLS operations
	err = config.LoadCaCert("", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	km, err := keys.NewLocalKeyManager("test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if km.IsSet() {
		if _, err = km.GetCurrentKeyFromCache(); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
	}
	if err = keys.Set(km); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	return &testInstance{
		Context:    ctx,
		Redis:      rd,
		Database:   pg,
		KeyManager: km,
		Config:     cfg,
		Conn:       conn,
		Logger:     logger,
	}
}

func buildApplication(t *testing.T) *applicationHandler {
	t.Helper()

	tl := newInfra(t)
	db := tl.Database

	logger := log.New("convoy", slog.LevelInfo)
	projectRepo := projects.New(logger, db)
	eventRepo := events.New(logger, db)
	configRepo := configuration.New(logger, db)
	eventDeliveryRepo := event_deliveries.New(logger, db)
	deliveryRepo := delivery_attempts.New(logger, db)

	app := &applicationHandler{
		projectRepo:       projectRepo,
		eventRepo:         eventRepo,
		configRepo:        configRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		deliveryRepo:      deliveryRepo,
		database:          db,
		redis:             tl.Redis,
		logger:            logger,
	}

	return app
}
