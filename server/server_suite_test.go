package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/frain-dev/convoy/auth/realm_chain"
	mcache "github.com/frain-dev/convoy/cache/memory"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	nooplimiter "github.com/frain-dev/convoy/limiter/noop"
	"github.com/frain-dev/convoy/logger"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/tracer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sebdah/goldie/v2"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

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

func getDB() *mongo.Database {

	db, _ := mongoStore.New(getConfig())

	_ = os.Setenv("TZ", "") // Use UTC by default :)

	return db.Client().(*mongo.Database)
}

func getQueueOptions() (queue.QueueOptions, error) {
	var opts queue.QueueOptions
	rC, qFn, err := redisqueue.NewClient(getConfig())
	if err != nil {
		return opts, err
	}
	opts = queue.QueueOptions{
		Type:    "redis",
		Name:    "EventQueue",
		Redis:   rC,
		Factory: qFn,
	}

	return opts, nil
}

func buildApplication() *applicationHandler {
	var tracer tracer.Tracer
	var db *mongo.Database
	var qOpts queue.QueueOptions

	db = getDB()
	qOpts, _ = getQueueOptions()

	groupRepo := mongoStore.NewGroupRepo(db)
	appRepo := mongoStore.NewApplicationRepo(db)
	eventRepo := mongoStore.NewEventRepository(db)
	eventDeliveryRepo := mongoStore.NewEventDeliveryRepository(db)
	apiKeyRepo := mongoStore.NewApiKeyRepo(db)
	eventQueue := redisqueue.NewQueue(qOpts)
	logger := logger.NewNoopLogger()
	cache := mcache.NewMemoryCache()
	limiter := nooplimiter.NewNoopLimiter()
	tracer = nil

	return newApplicationHandler(
		eventRepo, eventDeliveryRepo, appRepo,
		groupRepo, apiKeyRepo, eventQueue, logger,
		tracer, cache, limiter,
	)
}

func verifyMatch(t *testing.T, w httptest.ResponseRecorder) {
	g := goldie.New(
		t,
		goldie.WithDiffEngine(goldie.ColoredDiff),
	)
	g.Assert(t, t.Name(), w.Body.Bytes())
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

func stripTimestamp(t *testing.T, obj string, b *bytes.Buffer) *bytes.Buffer {
	var res serverResponse
	buf := b.Bytes()
	err := json.NewDecoder(b).Decode(&res)
	if err != nil {
		t.Errorf("could not stripTimestamp: %s", err)
		t.FailNow()
	}

	if res.Data == nil {
		return bytes.NewBuffer(buf)
	}

	switch obj {
	case "application":
		var a datastore.Application
		err := json.Unmarshal(res.Data, &a)
		if err != nil {
			t.Errorf("could not stripTimestamp: %s", err)
			t.FailNow()
		}

		a.UID = ""
		a.CreatedAt, a.UpdatedAt, a.DeletedAt = 0, 0, 0

		jsonData, err := json.Marshal(a)
		if err != nil {
			t.Error(err)
		}

		return bytes.NewBuffer(jsonData)
	case "group":
		var g datastore.Group
		err := json.Unmarshal(res.Data, &g)
		if err != nil {
			t.Errorf("could not stripTimestamp: %s", err)
			t.FailNow()
		}

		g.UID = ""
		g.CreatedAt, g.UpdatedAt, g.DeletedAt = 0, 0, 0

		jsonData, err := json.Marshal(g)
		if err != nil {
			t.Error(err)
		}

		return bytes.NewBuffer(jsonData)
	case "endpoint":
		var e datastore.Endpoint
		err := json.Unmarshal(res.Data, &e)
		if err != nil {
			t.Errorf("could not stripTimestamp: %s", err)
			t.FailNow()
		}

		e.UID = ""
		e.CreatedAt, e.UpdatedAt, e.DeletedAt = 0, 0, 0

		jsonData, err := json.Marshal(e)
		if err != nil {
			t.Error(err)
		}

		return bytes.NewBuffer(jsonData)
	case "apiKey":
		var e datastore.APIKey
		err := json.Unmarshal(res.Data, &e)
		if err != nil {
			t.Errorf("could not stripTimestamp: %s", err)
			t.FailNow()
		}

		e.UID = ""
		e.CreatedAt = 0
		e.ExpiresAt = 0

		jsonData, err := json.Marshal(e)
		if err != nil {
			t.Error(err)
		}

		return bytes.NewBuffer(jsonData)
	default:
		t.Errorf("invalid data body - %v of type %T", obj, obj)
		t.FailNow()
	}

	return nil
}
