package testenv

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"golang.org/x/sync/errgroup"
)

type Environment struct {
	CloneTestDatabase  PostgresDBCloneFunc
	NewRedisClient     RedisClientFunc
	NewQueueInspector  QueueInspectorFunc
	NewMinIOClient     MinIOClientFunc
	NewRabbitMQConnect RabbitMQConnectionFunc
}

func Launch(ctx context.Context) (*Environment, func() error, error) {
	pgcontainer, cloner, err := NewTestPostgres(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("start postgres container: %w", err)
	}

	rediscontainer, rcFactory, err := NewTestRedis(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("start redis container: %w", err)
	}

	miniocontainer, minioFactory, err := NewTestMinIO(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("start minio container: %w", err)
	}

	rabbitmqcontainer, rmqFactory, err := NewTestRabbitMQ(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("start rabbitmq container: %w", err)
	}

	// Get Redis address for queue inspector
	redisAddr, err := rediscontainer.ConnectionString(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("get redis address: %w", err)
	}

	inspectorFactory := newQueueInspectorFactory(redisAddr)

	res := &Environment{
		CloneTestDatabase:  cloner,
		NewRedisClient:     rcFactory,
		NewQueueInspector:  inspectorFactory,
		NewMinIOClient:     minioFactory,
		NewRabbitMQConnect: rmqFactory,
	}

	return res, func() error {
		var eg errgroup.Group
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := pgcontainer.Terminate(ctx); err != nil {
				log.Printf("terminate postgres container: %v", err)
			}
			return nil
		})
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := rediscontainer.Terminate(ctx); err != nil {
				log.Printf("terminate redis container: %v", err)
			}
			return nil
		})
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := miniocontainer.Terminate(ctx); err != nil {
				log.Printf("terminate minio container: %v", err)
			}
			return nil
		})
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := rabbitmqcontainer.Terminate(ctx); err != nil {
				log.Printf("terminate rabbitmq container: %v", err)
			}
			return nil
		})

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
