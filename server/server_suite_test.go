//go:build integration
// +build integration

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth/realm_chain"
	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	nooplimiter "github.com/frain-dev/convoy/limiter/noop"
	"github.com/frain-dev/convoy/logger"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	noopsearcher "github.com/frain-dev/convoy/searcher/noop"
	"github.com/frain-dev/convoy/tracer"
)

// TEST HELPERS.
func getMongoDSN() string {
	return os.Getenv("TEST_MONGO_DSN")
}

func getRedisDSN() string {
	return os.Getenv("TEST_REDIS_DSN")
}

func getConfig() config.Configuration {
	return config.Configuration{
		Queue: config.QueueConfiguration{
			Type: config.RedisQueueProvider,
			Redis: config.RedisQueueConfiguration{
				Dsn: getRedisDSN(),
			},
		},
		Database: config.DatabaseConfiguration{
			Type: config.MongodbDatabaseProvider,
			Dsn:  getMongoDSN(),
		},
	}
}

func getDB() datastore.DatabaseClient {

	db, _ := mongoStore.New(getConfig())

	_ = os.Setenv("TZ", "") // Use UTC by default :)

	return db.(*mongoStore.Client)
}

func getQueueOptions(name string) (queue.QueueOptions, error) {
	var opts queue.QueueOptions
	rC, err := redisqueue.NewClient(getConfig())
	if err != nil {
		return opts, err
	}
	opts = queue.QueueOptions{
		Type:  "redis",
		Name:  name,
		Redis: rC,
	}

	return opts, nil
}

func buildApplication() *applicationHandler {
	var tracer tracer.Tracer
	var qOpts, cOpts queue.QueueOptions
	var q queue.Queuer

	defaultOpts, _ = getQueueOptions("EventQueue")

	queue = redisqueue.NewQueuer(opts)

	db := getDB()
	cOpts, _ = getQueueOptions("CreateEventQueue")

	_ = queue.NewQueue(defaultOpts)
	_ = queue.NewQueue(cOpts)

	groupRepo := db.GroupRepo()
	appRepo := db.AppRepo()
	eventRepo := db.EventRepo()
	eventDeliveryRepo := db.EventDeliveryRepo()
	apiKeyRepo := db.APIRepo()
	sourceRepo := db.SourceRepo()
	logger := logger.NewNoopLogger()
	cache := ncache.NewNoopCache()
	limiter := nooplimiter.NewNoopLimiter()
	searcher := noopsearcher.NewNoopSearcher()
	tracer = nil

	return newApplicationHandler(
		eventRepo, eventDeliveryRepo, appRepo,
		groupRepo, apiKeyRepo, sourceRepo, queue,
		logger, tracer, cache, limiter, searcher,
	)
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository) {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo)
	if err != nil {
		t.Errorf("failed to initialize realm chain : %v", err)
	}
}

func parseResponse(t *testing.T, r *http.Response, object interface{}) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	var sR serverResponse
	err = json.Unmarshal(body, &sR)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = json.Unmarshal(sR.Data, object)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func randBool() bool {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(2) == 1
}

func createRequest(method string, url string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, url, body)
	req.SetBasicAuth("test", "test")
	req.Header.Add("Content-Type", "application/json")

	return req
}

func serialize(r string, args ...interface{}) io.Reader {
	v := fmt.Sprintf(r, args...)
	return strings.NewReader(v)
}
