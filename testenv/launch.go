package testenv

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/testcontainers/testcontainers-go"
	"golang.org/x/sync/errgroup"
)

// RabbitMQRestartFunc is a function type for restarting RabbitMQ container
type RabbitMQRestartFunc func(ctx context.Context) error

// LaunchOption is a function that configures which containers to launch
type LaunchOption func(*launchConfig)

// launchConfig holds the configuration for which containers to start
type launchConfig struct {
	enablePostgres   bool
	enableRedis      bool
	enableMinIO      bool
	enableRabbitMQ   bool
	enableLocalStack bool
	enableKafka      bool
	enablePubSub     bool
}

// defaultConfig returns the default configuration (Postgres + Redis only)
func defaultConfig() *launchConfig {
	return &launchConfig{
		enablePostgres:   true,
		enableRedis:      true,
		enableMinIO:      false,
		enableRabbitMQ:   false,
		enableLocalStack: false,
		enableKafka:      false,
		enablePubSub:     false,
	}
}

// WithoutPostgres disables PostgreSQL container
func WithoutPostgres() LaunchOption {
	return func(c *launchConfig) {
		c.enablePostgres = false
	}
}

// WithoutRedis disables Redis container
func WithoutRedis() LaunchOption {
	return func(c *launchConfig) {
		c.enableRedis = false
	}
}

// WithMinIO enables MinIO container
func WithMinIO() LaunchOption {
	return func(c *launchConfig) {
		c.enableMinIO = true
	}
}

// WithRabbitMQ enables RabbitMQ container
func WithRabbitMQ() LaunchOption {
	return func(c *launchConfig) {
		c.enableRabbitMQ = true
	}
}

// WithLocalStack enables LocalStack container
func WithLocalStack() LaunchOption {
	return func(c *launchConfig) {
		c.enableLocalStack = true
	}
}

// WithKafka enables Kafka container
func WithKafka() LaunchOption {
	return func(c *launchConfig) {
		c.enableKafka = true
	}
}

// WithPubSub enables Google Pub/Sub emulator container
func WithPubSub() LaunchOption {
	return func(c *launchConfig) {
		c.enablePubSub = true
	}
}

// launchState tracks which containers were actually started
type launchState struct {
	pgcontainer         testcontainers.Container
	rediscontainer      testcontainers.Container
	miniocontainer      testcontainers.Container
	rabbitmqcontainer   *RabbitMQContainer
	localstackcontainer testcontainers.Container
	kafkacontainer      testcontainers.Container
	pubsubcontainer     testcontainers.Container
}

type Environment struct {
	// Core dependencies (always present when enabled)
	CloneTestDatabase PostgresDBCloneFunc
	NewRedisClient    RedisClientFunc
	NewQueueInspector QueueInspectorFunc

	// Optional dependencies (nil if not started)
	NewMinIOClient        *MinIOClientFunc
	NewRabbitMQConnect    *RabbitMQConnectionFunc
	NewLocalStackConnect  *LocalStackConnectionFunc
	NewKafkaConnect       *KafkaConnectionFunc
	NewPubSubEmulatorHost *PubSubEmulatorHostFunc
	RestartRabbitMQ       *RabbitMQRestartFunc
}

func Launch(ctx context.Context, opts ...LaunchOption) (*Environment, func() error, error) {
	// Apply options to default config
	config := defaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	// Track started containers
	state := &launchState{}

	// Start PostgreSQL if enabled
	var cloner PostgresDBCloneFunc
	if config.enablePostgres {
		pgcontainer, cloneFunc, err := NewTestPostgres(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("start postgres container: %w", err)
		}
		state.pgcontainer = pgcontainer
		cloner = cloneFunc
	}

	// Start Redis if enabled
	var rcFactory RedisClientFunc
	var inspectorFactory QueueInspectorFunc
	if config.enableRedis {
		rediscontainer, redisFactory, err := NewTestRedis(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("start redis container: %w", err)
		}
		state.rediscontainer = rediscontainer
		rcFactory = redisFactory

		// Get Redis address for queue inspector
		redisAddr, err := rediscontainer.ConnectionString(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("get redis address: %w", err)
		}
		inspectorFactory = newQueueInspectorFactory(redisAddr)
	}

	// Start MinIO if enabled
	var minioFactory *MinIOClientFunc
	if config.enableMinIO {
		miniocontainer, factory, err := NewTestMinIO(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("start minio container: %w", err)
		}
		state.miniocontainer = miniocontainer
		factoryCopy := factory
		minioFactory = &factoryCopy
	}

	// Start RabbitMQ if enabled
	var rmqFactory *RabbitMQConnectionFunc
	var restartRabbitMQ *RabbitMQRestartFunc
	if config.enableRabbitMQ {
		rabbitmqcontainer, factory, err := NewTestRabbitMQ(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("start rabbitmq container: %w", err)
		}
		state.rabbitmqcontainer = rabbitmqcontainer
		factoryCopy := factory
		rmqFactory = &factoryCopy
		var restartFunc RabbitMQRestartFunc = rabbitmqcontainer.Restart
		restartRabbitMQ = &restartFunc
	}

	// Start LocalStack if enabled
	var localstackFactory *LocalStackConnectionFunc
	if config.enableLocalStack {
		localstackcontainer, factory, err := NewTestLocalStack(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("start localstack container: %w", err)
		}
		state.localstackcontainer = localstackcontainer
		factoryCopy := factory
		localstackFactory = &factoryCopy
	}

	// Start Kafka if enabled
	var kafkaFactory *KafkaConnectionFunc
	if config.enableKafka {
		kafkacontainer, factory, err := NewTestKafka(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("start kafka container: %w", err)
		}
		state.kafkacontainer = kafkacontainer
		factoryCopy := factory
		kafkaFactory = &factoryCopy
	}

	// Start Pub/Sub emulator if enabled
	var pubsubFactory *PubSubEmulatorHostFunc
	if config.enablePubSub {
		pubsubcontainer, factory, err := NewTestPubSubEmulator(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("start pubsub emulator container: %w", err)
		}
		state.pubsubcontainer = pubsubcontainer
		factoryCopy := factory
		pubsubFactory = &factoryCopy
	}

	res := &Environment{
		CloneTestDatabase:     cloner,
		NewRedisClient:        rcFactory,
		NewQueueInspector:     inspectorFactory,
		NewMinIOClient:        minioFactory,
		NewRabbitMQConnect:    rmqFactory,
		NewLocalStackConnect:  localstackFactory,
		NewKafkaConnect:       kafkaFactory,
		NewPubSubEmulatorHost: pubsubFactory,
		RestartRabbitMQ:       restartRabbitMQ,
	}

	return res, func() error {
		var eg errgroup.Group

		// Terminate PostgreSQL if it was started
		if state.pgcontainer != nil {
			eg.Go(func() error {
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if termErr := state.pgcontainer.Terminate(c); termErr != nil {
					slog.Info(fmt.Sprintf("terminate postgres container: %v", termErr))
				}
				return nil
			})
		}

		// Terminate Redis if it was started
		if state.rediscontainer != nil {
			eg.Go(func() error {
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if termErr := state.rediscontainer.Terminate(c); termErr != nil {
					slog.Info(fmt.Sprintf("terminate redis container: %v", termErr))
				}
				return nil
			})
		}

		// Terminate MinIO if it was started
		if state.miniocontainer != nil {
			eg.Go(func() error {
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if termErr := state.miniocontainer.Terminate(c); termErr != nil {
					slog.Info(fmt.Sprintf("terminate minio container: %v", termErr))
				}
				return nil
			})
		}

		// Terminate RabbitMQ if it was started
		if state.rabbitmqcontainer != nil {
			eg.Go(func() error {
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if termErr := state.rabbitmqcontainer.Terminate(c); termErr != nil {
					slog.Info(fmt.Sprintf("terminate rabbitmq container: %v", termErr))
				}
				return nil
			})
		}

		// Terminate LocalStack if it was started
		if state.localstackcontainer != nil {
			eg.Go(func() error {
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if termErr := state.localstackcontainer.Terminate(c); termErr != nil {
					slog.Info(fmt.Sprintf("terminate localstack container: %v", termErr))
				}
				return nil
			})
		}

		// Terminate Kafka if it was started
		if state.kafkacontainer != nil {
			eg.Go(func() error {
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if termErr := state.kafkacontainer.Terminate(c); termErr != nil {
					slog.Info(fmt.Sprintf("terminate kafka container: %v", termErr))
				}
				return nil
			})
		}

		// Terminate Pub/Sub emulator if it was started
		if state.pubsubcontainer != nil {
			eg.Go(func() error {
				c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if termErr := state.pubsubcontainer.Terminate(c); termErr != nil {
					slog.Info(fmt.Sprintf("terminate pubsub emulator container: %v", termErr))
				}
				return nil
			})
		}

		return eg.Wait()
	}, nil
}

// newQueueInspectorFactory creates a factory function for creating asynq inspectors
func newQueueInspectorFactory(redisAddr string) QueueInspectorFunc {
	return func(t *testing.T) *asynq.Inspector {
		t.Helper()

		// Parse the Redis connection string to extract host:port
		// testcontainers returns "redis://localhost:port" but asynq expects "localhost:port"
		uri, err := url.Parse(redisAddr)
		if err != nil {
			t.Fatalf("failed to parse redis connection string: %v", err)
		}

		redisOpt := asynq.RedisClientOpt{Addr: uri.Host}
		return asynq.NewInspector(redisOpt)
	}
}
