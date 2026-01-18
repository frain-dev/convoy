package task

import (
	"context"
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
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/projects"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testenv"
)

var (
	infra *testenv.Environment
)

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	infra = res

	code := m.Run()

	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}

type testInstance struct {
	Logger     *log.Logger
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
	logger.SetLevel(log.FatalLevel)

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
		log.Fatal(err)
	}

	km, err := keys.NewLocalKeyManager("test")
	if err != nil {
		log.Fatal(err)
	}
	if km.IsSet() {
		if _, err = km.GetCurrentKeyFromCache(); err != nil {
			log.Fatal(err)
		}
	}
	if err = keys.Set(km); err != nil {
		log.Fatal(err)
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

	projectRepo := projects.New(log.NewLogger(os.Stdout), db)
	eventRepo := postgres.NewEventRepo(db)
	configRepo := configuration.New(log.NewLogger(os.Stdout), db)
	eventDeliveryRepo := postgres.NewEventDeliveryRepo(db)
	deliveryRepo := delivery_attempts.New(log.NewLogger(os.Stdout), db)

	app := &applicationHandler{
		projectRepo:       projectRepo,
		eventRepo:         eventRepo,
		configRepo:        configRepo,
		eventDeliveryRepo: eventDeliveryRepo,
		deliveryRepo:      deliveryRepo,
		database:          db,
		redis:             tl.Redis,
	}

	return app
}
