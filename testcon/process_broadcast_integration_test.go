package testcon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/cmd/agent"
	"github.com/frain-dev/convoy/cmd/bootstrap"
	configCmd "github.com/frain-dev/convoy/cmd/config"
	"github.com/frain-dev/convoy/cmd/hooks"
	"github.com/frain-dev/convoy/cmd/ingest"
	"github.com/frain-dev/convoy/cmd/migrate"
	"github.com/frain-dev/convoy/cmd/retry"
	"github.com/frain-dev/convoy/cmd/server"
	"github.com/frain-dev/convoy/cmd/stream"
	"github.com/frain-dev/convoy/cmd/version"
	"github.com/frain-dev/convoy/cmd/worker"
	"github.com/frain-dev/convoy/config"
	dbhook "github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/listener"
	pgconvoy "github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	redisQueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/oklog/ulid/v2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIngestHappyPath(t *testing.T) {

	container, err := createPGContainer(t)
	if err != nil {
		t.Fatal(err)
	}
	port := getDbPort(t, container)

	rContainer, err := createRedisContainer()
	if err != nil {
		t.Fatal(err)
	}
	rPort := getRedisPort(t, rContainer)
	assert.NoError(t, err)

	bytesJSON, err := os.ReadFile("./testdata/config/convoy-test.json")
	if err != nil {
		log.Fatal(err)
	}

	var cfgSrc config.Configuration
	err = json.Unmarshal(bytesJSON, &cfgSrc)
	assert.NoError(t, err)
	cfgSrc.Database.Port = port
	cfgSrc.Redis.Port = rPort
	cfgSrc.Logger.Level = "info"

	cfgDest, err := json.MarshalIndent(cfgSrc, "", " ")
	assert.NoError(t, err)

	cfgFile, err := os.CreateTemp("", "convoy-test-mod.json")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Config file := ", cfgFile.Name())
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			log.Errorln(err)
		}
	}(cfgFile.Name())

	err = os.WriteFile(cfgFile.Name(), cfgDest, 0644)
	assert.NoError(t, err)

	var args = []string{"-"}
	cmd := loadConfigAndRunCommandOptional(port, rPort, cfgFile.Name(), args, false)
	cfgPath, err := cmd.Flags().GetString("config")
	assert.NoError(t, err)

	err = config.LoadConfig(cfgPath)
	assert.NoError(t, err)

	cfg, err := config.Get()
	assert.NoError(t, err)
	cfg.Database.Port = port
	cfg.Redis.Port = rPort
	cfg.ConsumerPoolSize = 100

	db, err := pgconvoy.NewDB(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer func(db *pgconvoy.Postgres) {
		err := db.Close()
		if err != nil {
			log.Errorln(err)
		}
	}(db)

	m := migrator.New(db)
	err = m.Up()
	if err != nil {
		log.Fatalf("migration up failed with error: %+v", err)
	}

	go func() {
		loadConfigAndRunCommand(port, rPort, cfgFile.Name(), []string{"worker"})
	}()

	go func() {
		loadConfigAndRunCommand(port, rPort, cfgFile.Name(), []string{"ingest"})
	}()

	q, err := setupQueueAndDb(t, err, cfg, db)

	publishIntermittently(t, err, q)

	testTimeout := time.After(3 * time.Minute)
	done := make(chan bool)
	checkDBPeriodically(&done, db)

	select {
	case <-testTimeout:
		t.Fatal("Test timed out!")
	case <-done:
	}

	finalCountQ := "select count(*) from convoy.event_deliveries"
	rows, err := db.GetDB().Queryx(finalCountQ)
	assert.NoError(t, err)
	var c counter
	for rows.Next() {
		err = rows.StructScan(&c)
		assert.NoError(t, err)
	}
	fmt.Println("Total events sent => ", c.Count)
	assert.True(t, c.Count >= 2_000, "events sent must be >= 2k")
}

func publishIntermittently(t *testing.T, err error, q queue.Queuer) {
	projId := "01HYWQJJ5ZH4H158E4RYHGNSDC"
	assert.NoError(t, err)
	go func() {
		for {

			broadcastEvent := &models.BroadcastEvent{
				EventType: "demo.test",
				ProjectID: projId,
				SourceID:  "",
				Data: json.RawMessage(`{
                              "userId": 1,
                              "id": 1,
                              "title": "delectus aut autem",
                              "completed": false
                            }`),
				CustomHeaders:  nil,
				IdempotencyKey: ulid.Make().String(),
			}

			eventByte, err := msgpack.EncodeMsgPack(broadcastEvent)
			assert.NoError(t, err)

			job := &queue.Job{
				ID:      ulid.Make().String(),
				Payload: eventByte,
			}

			// write to our queue if it's a broadcast event
			err = q.Write(convoy.CreateBroadcastEventProcessor, convoy.CreateEventQueue, job)
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}
	}()
}

func setupQueueAndDb(t *testing.T, err error, cfg config.Configuration, db *pgconvoy.Postgres) (queue.Queuer, error) {
	assert.NoError(t, err)
	var ca cache.Cache
	var q queue.Queuer

	r, err := rdb.NewClient(cfg.Redis.BuildDsn())
	assert.NoError(t, err)
	queueNames := map[string]int{
		string(convoy.EventQueue):       3,
		string(convoy.CreateEventQueue): 3,
		string(convoy.ScheduleQueue):    1,
		string(convoy.DefaultQueue):     1,
		string(convoy.MetaEventQueue):   1,
	}

	opts := queue.QueueOptions{
		Names:             queueNames,
		RedisClient:       r,
		RedisAddress:      cfg.Redis.BuildDsn(),
		Type:              string(config.RedisQueueProvider),
		PrometheusAddress: cfg.Prometheus.Dsn,
	}
	q = redisQueue.NewQueue(opts)

	ca, err = cache.NewCache(cfg.Redis)
	assert.NoError(t, err)

	h := dbhook.Init()

	projectListener := listener.NewProjectListener(q)
	h.RegisterHook(datastore.ProjectUpdated, projectListener.AfterUpdate)
	projectRepo := pgconvoy.NewProjectRepo(db, ca)

	metaEventRepo := pgconvoy.NewMetaEventRepo(db, ca)
	endpointListener := listener.NewEndpointListener(q, projectRepo, metaEventRepo)
	eventDeliveryListener := listener.NewEventDeliveryListener(q, projectRepo, metaEventRepo)

	h.RegisterHook(datastore.EndpointCreated, endpointListener.AfterCreate)
	h.RegisterHook(datastore.EndpointUpdated, endpointListener.AfterUpdate)
	h.RegisterHook(datastore.EndpointDeleted, endpointListener.AfterDelete)
	h.RegisterHook(datastore.EventDeliveryUpdated, eventDeliveryListener.AfterUpdate)
	return q, err
}

type counter = struct {
	Count int
}

func checkDBPeriodically(done *chan bool, p *pgconvoy.Postgres) {
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	var noResultsCtr atomic.Uint64
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("Checking db for data...")
				countQ := "select count(*) from convoy.event_deliveries"
				rows, err := p.GetDB().Queryx(countQ)
				if err != nil {
					panic(err)
				}
				for rows.Next() {
					var c counter
					err = rows.StructScan(&c)
					if err != nil {
						panic(err)
					}
					fmt.Println("Count ", c.Count)

					if c.Count > 1000 {
						fmt.Println("Ingest success")
						noResultsCtr.Store(0) // reset

						if c.Count > 2_000 {
							fmt.Println("Ingest completed successfully")
							*done <- true
							quit <- struct{}{}
							return
						}
					} else {
						noResultsCtr.Add(1)
						if noResultsCtr.Load() == 1000 {
							fmt.Println("Ingest error - no result after 10 checks")
							*done <- true
							return
						}
					}
				}
				err = rows.Close()
				if err != nil {
					panic(err)
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func getDbPort(t *testing.T, container *PostgresContainer) int {
	// postgres://postgres:postgres@localhost:52226/convoy-test-db?sslmode=disable
	portStrings := strings.Split(container.ConnectionString, "/convoy?")
	portStrings = strings.Split(portStrings[0], ":")
	port, err := strconv.Atoi(portStrings[len(portStrings)-1])
	assert.NoError(t, err)
	log.Info("PostgreSQL port: ", port)
	return port
}

func getRedisPort(t *testing.T, container *RedisContainer) int {
	portStrings := strings.Split(container.ConnectionString, ":")
	port, err := strconv.Atoi(portStrings[len(portStrings)-1])
	assert.NoError(t, err)
	log.Info("Redis port: ", port)
	return port
}

type PostgresContainer struct {
	*postgres.PostgresContainer
	ConnectionString string
}

func createPGContainer(t *testing.T) (*PostgresContainer, error) {
	ctx := context.Background()

	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:15.2-alpine"),
		postgres.WithInitScripts(
			filepath.Join(".", "testdata", "init-db.sql"),
		),
		postgres.WithDatabase("convoy"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate pgContainer: %s", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	assert.NoError(t, err)
	log.Info("Conn: " + connStr)
	return &PostgresContainer{
		PostgresContainer: pgContainer,
		ConnectionString:  connStr,
	}, nil
}

type RedisContainer struct {
	*redis.RedisContainer
	ConnectionString string
}

func createRedisContainer() (*RedisContainer, error) {
	ctx := context.Background()

	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("redis:6-alpine"),
	)
	if err != nil {
		log.Fatalf("failed to start container: %s", err)
	}

	uri, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("failed to get connection string: %s", err)
	}
	log.Info("Conn: ", redisContainer)

	return &RedisContainer{
		RedisContainer:   redisContainer,
		ConnectionString: uri,
	}, nil
}

func loadConfigAndRunCommand(pgPort int, redisPort int, configPath string, args []string) *cli.ConvoyCli {
	return loadConfigAndRunCommandOptional(pgPort, redisPort, configPath, args, true)
}
func loadConfigAndRunCommandOptional(pgPort int, rPort int, configPath string, args []string, execute bool) *cli.ConvoyCli {
	slog := logrus.New()
	slog.Out = os.Stdout

	app := &cli.App{}
	app.Version = convoy.GetVersionFromFS(convoy.F)
	db := &pgconvoy.Postgres{}

	c := cli.NewCli(app)

	var dbPort int
	var dbType string
	var dbHost string
	var dbScheme string
	var dbUsername string
	var dbPassword string
	var dbDatabase string

	var fflag string
	var enableProfiling bool

	var redisPort int
	var redisHost string
	var redisType string
	var redisScheme string
	var redisUsername string
	var redisPassword string
	var redisDatabase string

	var tracerType string
	var sentryDSN string
	var otelSampleRate float64
	var otelCollectorURL string
	var otelAuthHeaderName string
	var otelAuthHeaderValue string
	var metricsBackend string
	var prometheusMetricsSampleTime uint64

	var configFile string

	//c.Flags().StringVar(&configFile, "config", "./testdata/config/convoy-test-mod.json", "Configuration file for convoy")
	c.Flags().StringVar(&configFile, "config", configPath, "Configuration file for convoy")

	// db config
	c.Flags().StringVar(&dbHost, "db-host", "", "Database Host")
	c.Flags().StringVar(&dbType, "db-type", "", "Database provider")
	c.Flags().StringVar(&dbScheme, "db-scheme", "", "Database Scheme")
	c.Flags().StringVar(&dbUsername, "db-username", "", "Database Username")
	c.Flags().StringVar(&dbPassword, "db-password", "", "Database Password")
	c.Flags().StringVar(&dbDatabase, "db-database", "", "Database Database")
	c.Flags().StringVar(&dbDatabase, "db-options", "", "Database Options")
	c.Flags().IntVar(&dbPort, "db-port", pgPort, "Database Port")

	// redis config
	c.Flags().StringVar(&redisHost, "redis-host", "", "Redis Host")
	c.Flags().StringVar(&redisType, "redis-type", "", "Redis provider")
	c.Flags().StringVar(&redisScheme, "redis-scheme", "", "Redis Scheme")
	c.Flags().StringVar(&redisUsername, "redis-username", "", "Redis Username")
	c.Flags().StringVar(&redisPassword, "redis-password", "", "Redis Password")
	c.Flags().StringVar(&redisDatabase, "redis-database", "", "Redis database")
	c.Flags().IntVar(&redisPort, "redis-port", rPort, "Redis Port")

	c.Flags().StringVar(&fflag, "feature-flag", "", "Enable feature flags (experimental)")
	c.Flags().BoolVar(&enableProfiling, "enable-profiling", false, "Enable profiling")

	// tracing
	c.Flags().StringVar(&tracerType, "tracer-type", "", "Tracer backend, e.g. sentry, datadog or otel")
	c.Flags().StringVar(&sentryDSN, "sentry-dsn", "", "Sentry backend dsn")
	c.Flags().Float64Var(&otelSampleRate, "otel-sample-rate", 1.0, "OTel tracing sample rate")
	c.Flags().StringVar(&otelCollectorURL, "otel-collector-url", "", "OTel collector URL")
	c.Flags().StringVar(&otelAuthHeaderName, "otel-auth-header-name", "", "OTel backend auth header name")
	c.Flags().StringVar(&otelAuthHeaderValue, "otel-auth-header-value", "", "OTel backend auth header value")

	// metrics
	c.Flags().StringVar(&metricsBackend, "metrics-backend", "prometheus", "Metrics backend e.g. prometheus. ('experimental' feature flag level required")
	c.Flags().Uint64Var(&prometheusMetricsSampleTime, "metrics-prometheus-sample-time", 5, "Prometheus metrics sample time")

	c.PersistentPreRunE(hooks.PreRun(app, db))
	c.PersistentPostRunE(hooks.PostRun(app, db))

	c.AddCommand(version.AddVersionCommand())
	c.AddCommand(server.AddServerCommand(app))
	c.AddCommand(worker.AddWorkerCommand(app))
	c.AddCommand(retry.AddRetryCommand(app))
	c.AddCommand(migrate.AddMigrateCommand(app))
	c.AddCommand(configCmd.AddConfigCommand(app))
	c.AddCommand(stream.AddStreamCommand(app))
	c.AddCommand(ingest.AddIngestCommand(app))
	c.AddCommand(bootstrap.AddBootstrapCommand(app))
	c.AddCommand(agent.AddAgentCommand(app))

	buf := new(bytes.Buffer)

	cmd := c.GetCmd()
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	if execute {
		if err := c.Execute(); err != nil {
			slog.Fatal(err)
		}
	}

	return c
}
