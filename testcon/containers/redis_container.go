package containers

import (
	"context"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
)

type RedisContainer struct {
	*redis.RedisContainer
	ConnectionString string
}

func CreateRedisContainer() (*RedisContainer, error) {
	ctx := context.Background()

	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("redis:6-alpine"),
	)
	if err != nil {
		log.Fatalf("failed to start container: %s", err)
	}

	uri, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("failed to get connection string: %s", err)
	}
	log.Info("Conn: ", redisContainer)

	return &RedisContainer{
		RedisContainer:   redisContainer,
		ConnectionString: uri,
	}, nil
}
