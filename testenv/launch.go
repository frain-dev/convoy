package testenv

import (
	"context"
	"fmt"
	"log"
	"time"

	"golang.org/x/sync/errgroup"
)

type Environment struct {
	CloneTestDatabase PostgresDBCloneFunc
	NewRedisClient    RedisClientFunc
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

	res := &Environment{
		CloneTestDatabase: cloner,
		NewRedisClient:    rcFactory,
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

		return eg.Wait()
	}, nil
}
