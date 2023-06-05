//go:build integration
// +build integration

package api

import (
	"bytes"
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

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/util"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/cache"
	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
)

// TEST HELPERS.
func getConfig() config.Configuration {
	_ = os.Setenv("CONVOY_DB_HOST", "0.0.0.0")
	_ = os.Setenv("CONVOY_REDIS_SCHEME", "redis")
	_ = os.Setenv("CONVOY_REDIS_PORT", "6379")

	_ = os.Setenv("CONVOY_DB_HOST", "localhost")
	_ = os.Setenv("CONVOY_DB_SCHEME", "postgres")
	_ = os.Setenv("CONVOY_DB_USERNAME", "admin")
	_ = os.Setenv("CONVOY_DB_PASSWORD", "password")
	_ = os.Setenv("CONVOY_DB_DATABASE", "convoy")
	_ = os.Setenv("CONVOY_DB_OPTIONS", "&sslmode=disable")
	_ = os.Setenv("CONVOY_DB_PORT", "5432")

	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, _ := config.Get()

	return cfg
}

func getDB() database.Database {
	db, err := postgres.NewDB(getConfig())
	if err != nil {
		panic(fmt.Sprintf("failed to connect to db: %v", err))
	}
	_ = os.Setenv("TZ", "") // Use UTC by default :)

	dbHooks := hooks.Init()
	dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}) {})

	return db
}

func getQueueOptions() (queue.QueueOptions, error) {
	var opts queue.QueueOptions
	cfg := getConfig()
	redis, err := rdb.NewClient(cfg.Redis.BuildDsn())
	if err != nil {
		return opts, err
	}
	queueNames := map[string]int{
		string(convoy.SearchIndexQueue): 6,
		string(convoy.EventQueue):       2,
		string(convoy.CreateEventQueue): 2,
	}
	opts = queue.QueueOptions{
		Names:        queueNames,
		RedisClient:  redis,
		RedisAddress: cfg.Redis.BuildDsn(),
		Type:         string(config.RedisQueueProvider),
	}

	return opts, nil
}

func buildServer() *ApplicationHandler {
	var t tracer.Tracer = nil
	var logger *log.Logger
	var qOpts queue.QueueOptions

	db := getDB()
	qOpts, _ = getQueueOptions()

	newQueue := redisqueue.NewQueue(qOpts)
	logger = log.NewLogger(os.Stderr)
	logger.SetLevel(log.FatalLevel)

	noopCache := ncache.NewNoopCache()

	ah, _ := NewApplicationHandler(
		&types.APIOptions{
			DB:     db,
			Queue:  newQueue,
			Logger: logger,
			Tracer: t,
			Cache:  noopCache,
		})

	_ = ah.RegisterPolicy()

	return ah
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository, cache cache.Cache) {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, cache)
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
			return fmt.Errorf("failed to authenticate: reponse body: %s", w.Body.String())
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
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(2) == 1
}

func createRequest(method, url, auth string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", auth))

	return req
}

func serialize(r string, args ...interface{}) io.Reader {
	v := fmt.Sprintf(r, args...)
	return strings.NewReader(v)
}
