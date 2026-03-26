package hooks

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
)

func TestGetQueueOptions(t *testing.T) {
	t.Run("Standard Redis Configuration", func(t *testing.T) {
		cfg := &config.Configuration{
			Redis: config.RedisConfiguration{
				Scheme: "redis",
				Host:   "localhost",
				Port:   6379,
			},
		}

		var redis *rdb.Redis
		opts, err := getQueueOptions(cfg, redis)

		assert.NoError(t, err)
		assert.Nil(t, opts.RedisFailoverOpt)
		assert.Equal(t, []string{"redis://localhost:6379"}, opts.RedisAddress)
	})

	t.Run("Redis Sentinel Configuration", func(t *testing.T) {
		cfg := &config.Configuration{
			Redis: config.RedisConfiguration{
				Scheme:           "redis-sentinel",
				Addresses:        "sentinel1:26379,sentinel2:26379",
				MasterName:       "mymaster",
				Username:         "user",
				Password:         "pass",
				SentinelPassword: "sentinel_pass",
				Database:         "0",
			},
		}

		var redis *rdb.Redis
		opts, err := getQueueOptions(cfg, redis)

		assert.NoError(t, err)
		assert.NotNil(t, opts.RedisFailoverOpt)
		assert.Equal(t, "mymaster", opts.RedisFailoverOpt.MasterName)
		assert.Equal(t, []string{"sentinel1:26379", "sentinel2:26379"}, opts.RedisFailoverOpt.SentinelAddrs)
		assert.Equal(t, "user", opts.RedisFailoverOpt.Username)
		assert.Equal(t, "pass", opts.RedisFailoverOpt.Password)
		assert.Equal(t, "sentinel_pass", opts.RedisFailoverOpt.SentinelPassword)
		assert.Equal(t, 0, opts.RedisFailoverOpt.DB)
	})

	t.Run("Redis Sentinel Invalid Database", func(t *testing.T) {
		cfg := &config.Configuration{
			Redis: config.RedisConfiguration{
				Scheme:   "redis-sentinel",
				Database: "invalid",
			},
		}

		var redis *rdb.Redis
		_, err := getQueueOptions(cfg, redis)

		assert.Error(t, err)
	})
}
