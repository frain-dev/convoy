package rdb

import (
	"errors"
	"github.com/frain-dev/convoy/util"
	"github.com/newrelic/go-agent/v3/integrations/nrredis-v9"
	"github.com/redis/go-redis/v9"
)

// Redis is our wrapper logic to instrument redis calls
type Redis struct {
	addresses []string
	client    redis.UniversalClient
}

// NewClient is used to create new Redis type. This type
// encapsulates our interaction with redis and provides instrumentation with new relic.
func NewClient(addresses []string) (*Redis, error) {
	if len(addresses) == 0 {
		return nil, errors.New("redis addresses list cannot be empty")
	}

	for _, dsn := range addresses {
		if util.IsStringEmpty(dsn) {
			return nil, errors.New("dsn cannot be empty")
		}
	}

	var client redis.UniversalClient

	if len(addresses) == 1 {
		opts, err := redis.ParseURL(addresses[0])
		if err != nil {
			return nil, err
		}

		client = redis.NewClient(opts)
		// Add Instrumentation
		client.AddHook(nrredis.NewHook(opts))
	} else {
		client = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs: addresses,
		})
		// Add Instrumentation
		client.AddHook(nrredis.NewHook(nil))
	}

	return &Redis{addresses: addresses, client: client}, nil
}

// Client is to return underlying redis interface
func (r *Redis) Client() redis.UniversalClient {
	return r.client
}

// MakeRedisClient is used to fulfill asynq's interface
func (r *Redis) MakeRedisClient() interface{} {
	return r.client
}
