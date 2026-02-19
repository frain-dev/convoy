package testenv

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type RedisSentinelClientFunc func(t *testing.T, db int) (*redis.Client, string, error)

func NewTestRedisSentinel(ctx context.Context) (testcontainers.Container, RedisSentinelClientFunc, error) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp", "26379/tcp"},
		Cmd: []string{
			"/bin/sh",
			"-c",
			`redis-server --port 6379 --daemonize yes && \
echo 'port 26379' > /etc/sentinel.conf && \
echo 'sentinel monitor mymaster 127.0.0.1 6379 1' >> /etc/sentinel.conf && \
echo 'sentinel down-after-milliseconds mymaster 5000' >> /etc/sentinel.conf && \
echo 'sentinel failover-timeout mymaster 60000' >> /etc/sentinel.conf && \
redis-sentinel /etc/sentinel.conf`,
		},
		WaitingFor: wait.ForLog("monitor master mymaster 127.0.0.1 6379"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start redis sentinel container: %w", err)
	}

	return container, newRedisSentinelClientFunc(container), nil
}

func newRedisSentinelClientFunc(container testcontainers.Container) RedisSentinelClientFunc {
	return func(t *testing.T, db int) (*redis.Client, string, error) {
		t.Helper()
		ctx := context.Background()

		sentinelHost, err := container.Host(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get sentinel host: %w", err)
		}

		sentinelPort, err := container.MappedPort(ctx, "26379/tcp")
		if err != nil {
			return nil, "", fmt.Errorf("failed to get sentinel port: %w", err)
		}

		masterPort, err := container.MappedPort(ctx, "6379/tcp")
		if err != nil {
			return nil, "", fmt.Errorf("failed to get master port: %w", err)
		}

		fmt.Printf("Sentinel Host: %s, Sentinel Port: %s, Master Port: %s\n", sentinelHost, sentinelPort.Port(), masterPort.Port())

		sentinelAddr := fmt.Sprintf("%s:%s", sentinelHost, sentinelPort.Port())

		client := redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    "mymaster",
			SentinelAddrs: []string{sentinelAddr},
			DB:            db,
			DialTimeout:   5 * time.Second,
			ReadTimeout:   5 * time.Second,
			WriteTimeout:  5 * time.Second,
		})

		t.Cleanup(func() {
			if err := client.Close(); err != nil {
				t.Logf("failed to close redis sentinel client: %v", err)
			}
		})

		return client, sentinelAddr, nil
	}
}
