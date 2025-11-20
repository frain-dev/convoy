package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/cache"
	rcache "github.com/frain-dev/convoy/cache/redis"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/keys"
	noopLicenser "github.com/frain-dev/convoy/internal/pkg/license/noop"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/testenv"
	"github.com/frain-dev/convoy/util"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
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

	km, err := keys.NewLocalKeyManager()
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

func getQueueOptions(t *testing.T, cfg config.Configuration) (queue.QueueOptions, error) {
	t.Helper()

	var opts queue.QueueOptions

	rd, err := rdb.NewClient(cfg.Redis.BuildDsn())
	if err != nil {
		return opts, err
	}
	queueNames := map[string]int{
		string(convoy.EventQueue):         3,
		string(convoy.CreateEventQueue):   3,
		string(convoy.EventWorkflowQueue): 3,
		string(convoy.ScheduleQueue):      1,
		string(convoy.DefaultQueue):       1,
		string(convoy.StreamQueue):        1,
		string(convoy.MetaEventQueue):     1,
	}
	opts = queue.QueueOptions{
		RedisClient:  rd,
		Names:        queueNames,
		RedisAddress: cfg.Redis.BuildDsn(),
		Type:         string(config.RedisQueueProvider),
	}

	return opts, nil
}

func buildServer(t *testing.T) *ApplicationHandler {
	t.Helper()

	var qOpts queue.QueueOptions

	tl := newInfra(t)
	db := tl.Database

	qOpts, err := getQueueOptions(t, tl.Config)
	require.NoError(t, err)

	cfg := tl.Config

	newQueue := redisqueue.NewQueue(qOpts)

	noopCache := rcache.NewRedisCacheFromClient(tl.Redis)
	limiter := rlimiter.NewLimiterFromRedisClient(tl.Redis)

	ah, err := NewApplicationHandler(
		&types.APIOptions{
			DB:       db,
			Queue:    newQueue,
			Redis:    tl.Redis,
			Logger:   tl.Logger,
			Cache:    noopCache,
			FFlag:    fflag.NewFFlag([]string{string(fflag.Prometheus), string(fflag.FullTextSearch)}),
			Rate:     limiter,
			Licenser: noopLicenser.NewLicenser(),
			Cfg:      cfg,
		})
	require.NoError(t, err)

	err = ah.RegisterPolicy()
	require.NoError(t, err)

	return ah
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository, portalLinkRepo datastore.PortalLinkRepository, cache cache.Cache) {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	logger := log.NewLogger(os.Stderr)
	logger.SetLevel(log.FatalLevel)

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
