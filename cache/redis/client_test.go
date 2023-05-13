//go:build integration
// +build integration

package rcache

import (
	"context"
	"github.com/frain-dev/convoy/config"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type data struct {
	Name string
}

func getDSN() string {
	c := config.RedisConfiguration{
		Scheme: "redis",
		Host:   os.Getenv("CONVOY_REDIS_HOST"),
		Port:   6379,
	}
	return c.BuildDsn()
}

const key = "test_key"

func Test_WriteToCache(t *testing.T) {
	cache, err := NewRedisCache(getDSN())
	require.NoError(t, err)

	err = cache.Set(context.TODO(), key, &data{Name: "test_name"}, 10*time.Second)
	require.NoError(t, err)
}

func Test_ReadFromCache(t *testing.T) {
	cache, err := NewRedisCache(getDSN())
	require.NoError(t, err)

	err = cache.Set(context.TODO(), key, &data{Name: "test_name"}, 10*time.Second)
	require.NoError(t, err)

	var item data
	err = cache.Get(context.TODO(), key, &item)

	require.NoError(t, err)
	require.Equal(t, "test_name", item.Name)
}

func Test_DeleteFromCache(t *testing.T) {
	cache, err := NewRedisCache(getDSN())
	require.NoError(t, err)

	err = cache.Set(context.TODO(), key, &data{Name: "test_name"}, 10*time.Second)
	require.NoError(t, err)

	err = cache.Delete(context.TODO(), key)
	require.NoError(t, err)

	var item data
	err = cache.Get(context.TODO(), key, &item)

	require.Equal(t, "", item.Name)
}
