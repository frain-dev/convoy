package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/broker"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/internal/pkg/license"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	"github.com/frain-dev/convoy/internal/portal_links"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/testenv"
	"github.com/frain-dev/convoy/util"
)

var (
	infra *testenv.Environment
)

func TestMain(m *testing.M) {
	_ = os.Setenv("CONVOY_JWT_SECRET", "test-access-secret")
	_ = os.Setenv("CONVOY_JWT_REFRESH_SECRET", "test-refresh-secret")

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

	// Point the config at the containers this test actually uses. Without this
	// it still describes localhost defaults, so anything built from the config
	// rather than from the handles below (the broker's dedicated advisory lock
	// pool, for one) would connect somewhere other than the database under test.
	cfg.Database.DSN = conn.Config().ConnString()
	if host, port, splitErr := splitHostPort(rd.Options().Addr); splitErr == nil {
		cfg.Redis.Scheme = config.RedisScheme
		cfg.Redis.Host = host
		cfg.Redis.Port = port
	}
	cfg.QueueProvider = testQueueProvider()

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

// testQueueProvider selects the broker the API suite runs against. The suite
// asserts provider-neutral behaviour, so the same tests can be run a second
// time against Postgres to prove parity instead of assuming it.
func testQueueProvider() config.QueueProvider {
	if p := os.Getenv("CONVOY_TEST_QUEUE_PROVIDER"); p != "" {
		return config.QueueProvider(p)
	}
	return config.RedisQueueProvider
}

func splitHostPort(addr string) (string, int, error) {
	host, rawPort, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

// newBroker builds the queue, cache and rate limiter a test server runs on
// through the same registry the binary uses, so the suite exercises real wiring
// instead of a hand-rolled stack that can drift from it.
func newBroker(t *testing.T, cfg config.Configuration, db database.Database, logger log.Logger) *broker.Dependencies {
	t.Helper()

	if cfg.QueueProvider == config.PostgresQueueProvider {
		cfg.EnableFeatureFlag = append(cfg.EnableFeatureFlag, string(fflag.PostgresQueue))
	}

	deps, err := broker.New(cfg, db.GetDB(), logger)
	require.NoError(t, err)
	t.Cleanup(func() { _ = deps.Close() })

	return deps
}

// newAPIOptions wires the full broker dependency set via ApplyToAPIOptions. The
// handler used to derive circuit breaker, trial cap, acker and friends from
// APIOptions.Redis; they are explicit now and anything left nil is silently
// skipped by its nil guard. ApplyToAPIOptions keeps the suite aligned with prod
// wiring so a partial build cannot run green without touching broker paths.
func newAPIOptions(tl *testInstance, deps *broker.Dependencies, licenser license.Licenser) *types.APIOptions {
	db := tl.Database
	o := &types.APIOptions{
		DB:                         db,
		Logger:                     tl.Logger,
		FFlag:                      fflag.NewFFlag([]string{string(fflag.Prometheus), string(fflag.FullTextSearch)}),
		FeatureFlagFetcher:         postgres.NewFeatureFlagFetcher(db),
		EarlyAdopterFeatureFetcher: postgres.NewEarlyAdopterFeatureFetcher(db),
		ConfigRepo:                 configuration.New(tl.Logger, db),
		Licenser:                   licenser,
		Cfg:                        tl.Config,
	}
	deps.ApplyToAPIOptions(o)
	return o
}

func buildServer(t *testing.T) *ApplicationHandler {
	t.Helper()

	tl := newInfra(t)
	deps := newBroker(t, tl.Config, tl.Database, tl.Logger)

	ah, err := NewApplicationHandler(newAPIOptions(tl, deps, noopLicenser.NewLicenser()))
	require.NoError(t, err)

	err = ah.RegisterPolicy()
	require.NoError(t, err)

	return ah
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository, portalLinkRepo *portal_links.Service, cache cache.Cache) {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	logger := log.New("convoy", log.LevelInfo)

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, cache, logger)
	if err != nil {
		t.Errorf("failed to initialize realm chain : %v", err)
	}
}

func parseResponse(t *testing.T, w *http.Response, object interface{}) {
	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	var sR util.ServerResponse
	err = json.Unmarshal(body, &sR)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = json.Unmarshal(sR.Data, object)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

type AuthenticatorFn func(r *http.Request, router http.Handler) error

func authenticateRequest(auth *models.LoginUser) AuthenticatorFn {
	return func(r *http.Request, router http.Handler) error {
		body, err := json.Marshal(auth)
		if err != nil {
			return err
		}

		req := createRequest(http.MethodPost, "/ui/auth/login", "", bytes.NewBuffer(body))
		w := httptest.NewRecorder()

		// Act
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			return fmt.Errorf("failed to authenticate: response body: %s", w.Body.String())
		}

		loginResp := &models.LoginUserResponse{}
		resp := &struct {
			Data interface{} `json:"data"`
		}{
			Data: loginResp,
		}
		err = json.NewDecoder(w.Body).Decode(resp)
		if err != nil {
			return err
		}

		r.Header.Set("Authorization", fmt.Sprintf("BEARER %s", loginResp.Token.AccessToken))
		return nil
	}
}

func randBool() bool {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	return rnd.Intn(2) == 1
}

func createRequest(method, url, auth string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth))
	req.Header.Add(VersionHeader, config.DefaultAPIVersion)

	return req
}

func serialize(r string, args ...interface{}) io.Reader {
	v := fmt.Sprintf(r, args...)
	return strings.NewReader(v)
}
