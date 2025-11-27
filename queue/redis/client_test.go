package redis

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/testenv"
)

var testInfra *testenv.Environment

func TestMain(m *testing.M) {
	res, cleanup, err := testenv.Launch(context.Background())
	if err != nil {
		log.Fatalf("Failed to launch test infrastructure: %v", err)
	}

	testInfra = res

	code := m.Run()

	if err := cleanup(); err != nil {
		log.Fatalf("Failed to cleanup test infrastructure: %v", err)
	}

	os.Exit(code)
}

type RedisQueueIntegrationTestSuite struct {
	suite.Suite
	eventQueue queue.Queuer
}

func (s *RedisQueueIntegrationTestSuite) SetupTest() {
	// Each test gets a fresh Redis client
	redisClient, err := testInfra.NewRedisClient(s.T(), 0)
	s.Require().NoError(err)

	// Flush the database to ensure a clean state
	err = redisClient.FlushDB(context.Background()).Err()
	s.Require().NoError(err)

	queueNames := map[string]int{
		string(convoy.EventQueue):         2,
		string(convoy.CreateEventQueue):   2,
		string(convoy.EventWorkflowQueue): 2,
	}

	// Load config for Redis DSN from the test container
	err = config.LoadConfig("")
	s.Require().NoError(err)
	cfg, err := config.Get()
	s.Require().NoError(err)

	// Create rdb.Redis wrapper for the queue
	rdbClient, err := rdb.NewClient(cfg.Redis.BuildDsn())
	s.Require().NoError(err)

	opts := queue.QueueOptions{
		Names:        queueNames,
		RedisClient:  rdbClient,
		RedisAddress: cfg.Redis.BuildDsn(),
		Type:         string(config.RedisQueueProvider),
	}

	s.eventQueue = NewQueue(opts)
}

func (s *RedisQueueIntegrationTestSuite) TestWrite() {
	tests := []struct {
		name            string
		queueName       string
		endpointID      string
		eventID         string
		eventDeliveryID string
	}{
		{
			name:            "write a single event to queue",
			queueName:       ulid.Make().String(),
			endpointID:      ulid.Make().String(),
			eventID:         ulid.Make().String(),
			eventDeliveryID: ulid.Make().String(),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			eventDelivery := &datastore.EventDelivery{
				UID: tc.eventDeliveryID,
			}
			job := &queue.Job{
				Payload: json.RawMessage(eventDelivery.UID),
			}

			taskName := convoy.TaskName(ulid.Make().String())
			err := s.eventQueue.Write(taskName, convoy.EventQueue, job)
			s.Require().NoError(err, "Failed to write to queue")
		})
	}
}

func TestRedisQueueIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(RedisQueueIntegrationTestSuite))
}
