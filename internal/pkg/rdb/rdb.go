package rdb

import (
	"errors"

	"github.com/frain-dev/convoy/util"
	"github.com/newrelic/go-agent/v3/integrations/nrredis-v9"
	"github.com/redis/go-redis/v9"
)

// Redis is our wrapper logic to instrument redis calls
type Redis struct {
	dsn    string
	client *redis.Client
}

// NewClient is used to create new Redis type. This type
// encapsulates our interaction with redis and provides instrumentation with new relic.
func NewClient(dsn string) (*Redis, error) {
	if util.IsStringEmpty(dsn) {
		return nil, errors.New("redis dsn cannot be empty")
	}

	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	// Add Instrumentation
	client.AddHook(nrredis.NewHook(opts))

	return &Redis{dsn: dsn, client: client}, nil
}

// Client is to return underlying redis interface
func (r *Redis) Client() *redis.Client {
	return r.client
}

// MakeRedisClient is used to fulfill asynq's interface
func (r *Redis) MakeRedisClient() interface{} {
	return r.client
}
