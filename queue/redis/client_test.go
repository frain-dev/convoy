package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/cache"
	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/limiter"
	nooplimiter "github.com/frain-dev/convoy/limiter/noop"
	"github.com/frain-dev/convoy/logger"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/searcher"
	noopsearcher "github.com/frain-dev/convoy/searcher/noop"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/frain-dev/convoy/tracer"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
	disqredis "github.com/frain-dev/disq/brokers/redis"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProducerIntegrationTestSuite struct {
	suite.Suite
	RedisClient   *redis.Client
	Event         *datastore.Event
	EventDelivery *datastore.EventDelivery
	DefaultGroup  *datastore.Group
	Queuer        queue.Queuer
	QueuerName    string
	DB            datastore.DatabaseClient
	ConvoyApp     *applicationHandler
}

func (p *ProducerIntegrationTestSuite) SetupSuite() {
	p.DB = getDB()
	p.ConvoyApp = buildApplication()
}

func (p *ProducerIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(p.DB)

	p.DefaultGroup, _ = testdb.SeedDefaultGroup(p.DB)
	app, _ := testdb.SeedApplication(p.DB, p.DefaultGroup, uuid.NewString(), "", false)
	p.Event, _ = testdb.SeedEvent(p.DB, app, p.DefaultGroup.UID, uuid.NewString(), "*", []byte(`{}`))
	p.EventDelivery, _ = seedEventDelivery(p.DB, app, &datastore.Event{}, &datastore.Endpoint{}, p.DefaultGroup.UID, uuid.NewString(), datastore.SuccessEventStatus)

	p.RedisClient, _ = NewClient(getConfig())
	a := p.ConvoyApp
	p.QueuerName = uuid.NewString()
	p.Queuer = NewQueuer(queue.QueueOptions{
		Name:  p.QueuerName,
		Redis: p.RedisClient,
	})

	p.Queuer.NewQueue(queue.QueueOptions{
		Name:  p.QueuerName,
		Redis: p.RedisClient,
	})

	handler1 := task.ProcessEventCreated(a.applicationRepo, a.eventRepo, a.groupRepo, a.eventDeliveryRepo, a.cache, p.Queuer)
	handler2 := task.ProcessEventDelivery(a.applicationRepo, a.eventDeliveryRepo, a.groupRepo, a.limiter)
	err := task.CreateTasks(a.groupRepo, convoy.CreateEventProcessor, handler1)
	require.NoError(p.T(), err)
	err = task.CreateTasks(a.groupRepo, convoy.EventProcessor, handler2)
	require.NoError(p.T(), err)
	err = config.LoadConfig("../testdata/convoy_redis.json")
	require.NoError(p.T(), err)
	initRealmChain(p.T(), p.DB.APIRepo())
}

func (s *ProducerIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (p *ProducerIntegrationTestSuite) Test_StartAll() {
	taskName := convoy.EventProcessor.SetPrefix(p.DefaultGroup.Name)
	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})
	payload := json.RawMessage(p.EventDelivery.UID)

	job := &queue.Job{
		Payload: payload,
		Delay:   0,
	}
	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})
	ctx := context.Background()
	_ = redisQueuer.Write(ctx, string(taskName), q1, job)
	_ = redisQueuer.Write(ctx, string(taskName), q2, job)
	_ = redisQueuer.Write(ctx, string(taskName), q3, job)

	redisQueuer.StartAll(ctx)
	time.Sleep(time.Duration(1) * time.Second)

	l1, _ := redisQueuer.Length(q1)
	l2, _ := redisQueuer.Length(q2)
	l3, _ := redisQueuer.Length(q3)
	_ = redisQueuer.StopAll()
	require.Equal(p.T(), []int{0, 0, 0}, []int{l1, l2, l3})
}

func (p *ProducerIntegrationTestSuite) Test_StartOne() {
	taskName := convoy.EventProcessor.SetPrefix(p.DefaultGroup.Name)

	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})
	payload := json.RawMessage(p.EventDelivery.UID)
	job := &queue.Job{
		Payload: payload,
		Delay:   0,
	}
	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})
	ctx := context.Background()
	_ = redisQueuer.Write(ctx, string(taskName), q1, job)
	_ = redisQueuer.Write(ctx, string(taskName), q2, job)
	_ = redisQueuer.Write(ctx, string(taskName), q3, job)

	_ = redisQueuer.StartOne(ctx, q1)
	time.Sleep(time.Duration(1) * time.Second)

	l1, _ := redisQueuer.Length(q1)
	l2, _ := redisQueuer.Length(q2)
	l3, _ := redisQueuer.Length(q3)
	_ = redisQueuer.StopAll()
	require.Equal(p.T(), []int{0, 1, 1}, []int{l1, l2, l3})
}

func (p *ProducerIntegrationTestSuite) Test_SendEvent() {
	taskName := convoy.CreateEventProcessor.SetPrefix(p.DefaultGroup.Name)
	event, _ := json.Marshal(p.Event)
	payload := json.RawMessage(event)

	job := &queue.Job{
		Err:     nil,
		Payload: payload,
		Delay:   0,
	}
	q1 := uuid.NewString()
	_ = p.Queuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})

	ctx := context.Background()

	_ = p.Queuer.Write(ctx, string(taskName), q1, job)
	p.Queuer.StartOne(ctx, q1)
	p.Queuer.StartOne(ctx, p.QueuerName)

	time.Sleep(time.Duration(1) * time.Second)
	s1, _ := p.Queuer.Stats(q1)
	s2, _ := p.Queuer.Stats(p.QueuerName)
	_ = p.Queuer.StopAll()
	require.Equal(p.T(), []int{1, 0}, []int{s1.Processed, s2.Processed})
}

func (p *ProducerIntegrationTestSuite) Test_StopAll() {
	taskName := convoy.EventProcessor.SetPrefix(p.DefaultGroup.Name)

	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})
	payload := json.RawMessage(p.EventDelivery.UID)

	job := &queue.Job{
		Payload: payload,
		Delay:   0,
	}
	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})
	ctx := context.Background()
	_ = redisQueuer.Write(ctx, string(taskName), q1, job)
	_ = redisQueuer.Write(ctx, string(taskName), q2, job)
	_ = redisQueuer.Write(ctx, string(taskName), q3, job)

	redisQueuer.StartAll(ctx)
	time.Sleep(time.Duration(1) * time.Second)
	_ = redisQueuer.StopAll()

	l1, _ := redisQueuer.(*RedisQueuer).Load(q1)
	l2, _ := redisQueuer.(*RedisQueuer).Load(q2)
	l3, _ := redisQueuer.(*RedisQueuer).Load(q3)

	require.Equal(p.T(), []bool{false, false, false}, []bool{l1.Status(), l2.Status(), l3.Status()})
}

func (p *ProducerIntegrationTestSuite) Test_StopOne() {
	taskName := convoy.EventProcessor.SetPrefix(p.DefaultGroup.Name)

	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})
	payload := json.RawMessage(p.EventDelivery.UID)

	job := &queue.Job{
		Payload: payload,
		Delay:   0,
	}
	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})
	ctx := context.Background()
	_ = redisQueuer.Write(ctx, string(taskName), q1, job)
	_ = redisQueuer.Write(ctx, string(taskName), q2, job)
	_ = redisQueuer.Write(ctx, string(taskName), q3, job)

	redisQueuer.StartAll(ctx)
	time.Sleep(time.Duration(1) * time.Second)
	_ = redisQueuer.StopOne(q2)

	l1, _ := redisQueuer.(*RedisQueuer).Load(q1)
	l2, _ := redisQueuer.(*RedisQueuer).Load(q2)
	l3, _ := redisQueuer.(*RedisQueuer).Load(q3)

	require.Equal(p.T(), []bool{true, false, true}, []bool{l1.Status(), l2.Status(), l3.Status()})
}

func (p *ProducerIntegrationTestSuite) Test_New_Queue_That_Exists() {

	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})

	q1 := "Testqueue"
	q2 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	err := redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})

	require.Equal(p.T(), fmt.Errorf("queue with name=%q already exists", "Testqueue"), err)

}

func (p *ProducerIntegrationTestSuite) Test_UpdateOne_That_Exists() {

	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})

	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})

	err := redisQueuer.Update(context.Background(), queue.QueueOptions{
		Name:        q1,
		Concurrency: 50,
	})
	require.NoError(p.T(), err)

	b1, _ := redisQueuer.(*RedisQueuer).Load(q1)
	require.Equal(p.T(), 50, int(b1.Config().(*disqredis.RedisConfig).Concurency))
}

func (p *ProducerIntegrationTestSuite) Test_UpdateOne_New() {

	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})

	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})

	_ = redisQueuer.Update(context.Background(), queue.QueueOptions{
		Name:        q2,
		Concurrency: 50,
	})

	b1, _ := redisQueuer.(*RedisQueuer).Load(q2)

	require.Equal(p.T(), 50, int(b1.Config().(*disqredis.RedisConfig).Concurency))
}

func (p *ProducerIntegrationTestSuite) Test_Delete() {

	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})

	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})

	_ = redisQueuer.Delete(q2)

	val := redisQueuer.Contains(q2)

	require.Equal(p.T(), false, val)
}

func (p *ProducerIntegrationTestSuite) Test_Stats() {
	taskName := convoy.EventProcessor.SetPrefix(p.DefaultGroup.Name)
	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  uuid.NewString(),
		Redis: p.RedisClient,
	})
	payload := json.RawMessage(p.EventDelivery.UID)

	job := &queue.Job{
		Payload: payload,
		Delay:   0,
	}
	q1 := uuid.NewString()
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q3,
	})
	ctx := context.Background()
	_ = redisQueuer.Write(ctx, string(taskName), q1, job)
	_ = redisQueuer.Write(ctx, string(taskName), q2, job)
	_ = redisQueuer.Write(ctx, string(taskName), q3, job)

	redisQueuer.StartAll(ctx)
	time.Sleep(time.Duration(1) * time.Second)

	s1, _ := redisQueuer.Stats(q1)
	s2, _ := redisQueuer.Stats(q2)
	s3, _ := redisQueuer.Stats(q3)
	_ = redisQueuer.StopAll()

	require.Equal(p.T(), []int{1, 1, 1}, []int{s1.Processed, s2.Processed, s3.Processed})
}

func (p *ProducerIntegrationTestSuite) Test_Write_Name_Doesnt_Exist() {
	taskName := convoy.EventProcessor.SetPrefix(p.DefaultGroup.Name)
	defaultQueue := uuid.NewString()
	redisQueuer := NewQueuer(queue.QueueOptions{
		Name:  defaultQueue,
		Redis: p.RedisClient,
	})
	payload := json.RawMessage(p.EventDelivery.UID)

	job := &queue.Job{
		Payload: payload,
		Delay:   0,
	}
	q1 := defaultQueue
	q2 := uuid.NewString()
	q3 := uuid.NewString()

	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q1,
	})
	_ = redisQueuer.NewQueue(queue.QueueOptions{
		Name: q2,
	})

	ctx := context.Background()
	_ = redisQueuer.Write(ctx, string(taskName), q1, job)
	_ = redisQueuer.Write(ctx, string(taskName), q2, job)
	_ = redisQueuer.Write(ctx, string(taskName), q3, job)

	l1, _ := redisQueuer.Length(q1)
	l2, _ := redisQueuer.Length(q2)
	_ = redisQueuer.StopAll()
	require.Equal(p.T(), []int{2, 1}, []int{l1, l2})
}

func Test_ProducerIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ProducerIntegrationTestSuite))
}

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

func buildApplication() *applicationHandler {

	db := getDB()

	logger := logger.NewNoopLogger()

	app := &applicationHandler{
		groupRepo:         db.GroupRepo(),
		applicationRepo:   db.AppRepo(),
		eventRepo:         db.EventRepo(),
		eventDeliveryRepo: db.EventDeliveryRepo(),
		apiKeyRepo:        db.APIRepo(),
		sourceRepo:        db.SourceRepo(),
		logger:            logger,
		cache:             ncache.NewNoopCache(),
		limiter:           nooplimiter.NewNoopLimiter(),
		searcher:          noopsearcher.NewNoopSearcher(),
		tracer:            nil,
	}

	return app
}

type applicationHandler struct {
	apiKeyRepo        datastore.APIKeyRepository
	groupRepo         datastore.GroupRepository
	applicationRepo   datastore.ApplicationRepository
	eventRepo         datastore.EventRepository
	eventDeliveryRepo datastore.EventDeliveryRepository
	sourceRepo        datastore.SourceRepository
	logger            logger.Logger
	tracer            tracer.Tracer
	cache             cache.Cache
	limiter           limiter.RateLimiter
	searcher          searcher.Searcher
}

func seedEventDelivery(db datastore.DatabaseClient, app *datastore.Application, event *datastore.Event, endpoint *datastore.Endpoint, groupID string, uid string, status datastore.EventDeliveryStatus) (*datastore.EventDelivery, error) {
	if util.IsStringEmpty(uid) {
		uid = uuid.New().String()
	}

	eventDelivery := &datastore.EventDelivery{
		UID: uid,
		EventMetadata: &datastore.EventMetadata{
			UID:       event.UID,
			EventType: event.EventType,
		},
		EndpointMetadata: &datastore.EndpointMetadata{
			UID:               endpoint.UID,
			TargetURL:         endpoint.TargetURL,
			Status:            endpoint.Status,
			Secret:            endpoint.Secret,
			HttpTimeout:       endpoint.HttpTimeout,
			RateLimit:         endpoint.RateLimit,
			RateLimitDuration: endpoint.RateLimitDuration,
			Sent:              false,
		},
		Status: status,
		AppMetadata: &datastore.AppMetadata{
			UID:          app.UID,
			Title:        app.Title,
			GroupID:      groupID,
			SupportEmail: app.SupportEmail,
		},
		Metadata: &datastore.Metadata{
			Data:            event.Data,
			Strategy:        datastore.DefaultStrategyConfig.Type,
			NumTrials:       0,
			IntervalSeconds: 2,
			RetryLimit:      2,
			NextSendTime:    primitive.NewDateTimeFromTime(time.Now()),
		},
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	// Seed Data.
	err := db.EventDeliveryRepo().CreateEventDelivery(context.TODO(), eventDelivery)
	if err != nil {
		return nil, err
	}

	return eventDelivery, nil
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
