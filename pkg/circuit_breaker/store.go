package circuit_breaker

import (
	"context"
	"errors"
	"fmt"
	"github.com/frain-dev/convoy/pkg/clock"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/redis/go-redis/v9"
	"strings"
	"sync"
	"time"
)

type CircuitBreakerStore interface {
	Lock(ctx context.Context, lockKey string, expiry uint64) (*redsync.Mutex, error)
	Unlock(ctx context.Context, mutex *redsync.Mutex) error
	Keys(context.Context, string) ([]string, error)
	GetOne(context.Context, string) (string, error)
	GetMany(context.Context, ...string) ([]interface{}, error)
	SetOne(context.Context, string, interface{}, time.Duration) error
	SetMany(context.Context, map[string]CircuitBreaker, time.Duration) error
}

type RedisStore struct {
	redis redis.UniversalClient
	clock clock.Clock
}

func NewRedisStore(redis redis.UniversalClient, clock clock.Clock) *RedisStore {
	return &RedisStore{
		redis: redis,
		clock: clock,
	}
}

func (s *RedisStore) Lock(ctx context.Context, mutexKey string, expiry uint64) (*redsync.Mutex, error) {
	pool := goredis.NewPool(s.redis)
	rs := redsync.New(pool)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	mutex := rs.NewMutex(mutexKey, redsync.WithExpiry(time.Duration(expiry)*time.Second), redsync.WithTries(1))
	err := mutex.LockContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain lock: %v", err)
	}

	return mutex, nil
}

func (s *RedisStore) Unlock(ctx context.Context, mutex *redsync.Mutex) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	ok, err := mutex.UnlockContext(ctx)
	if !ok {
		return fmt.Errorf("failed to release lock: %v", err)
	}

	if err != nil {
		return fmt.Errorf("failed to release lock: %v", err)
	}

	return nil
}

// Keys returns all the keys used by the circuit breaker store
func (s *RedisStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	return s.redis.Keys(ctx, fmt.Sprintf("%s*", pattern)).Result()
}

func (s *RedisStore) GetOne(ctx context.Context, key string) (string, error) {
	key, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrCircuitBreakerNotFound
		}
		return "", err
	}
	return key, nil
}

func (s *RedisStore) GetMany(ctx context.Context, keys ...string) ([]any, error) {
	res, err := s.redis.MGet(ctx, keys...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []any{}, nil
		}
		return nil, err
	}

	return res, nil
}

func (s *RedisStore) SetOne(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return s.redis.Set(ctx, key, value, expiration).Err()
}

func (s *RedisStore) SetMany(ctx context.Context, breakers map[string]CircuitBreaker, ttl time.Duration) error {
	pipe := s.redis.TxPipeline()
	for key, breaker := range breakers {
		val := breaker.String()
		if innerErr := pipe.Set(ctx, key, val, ttl).Err(); innerErr != nil {
			return innerErr
		}
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

type TestStore struct {
	store map[string]CircuitBreaker
	mu    *sync.RWMutex
	clock clock.Clock
}

func NewTestStore() *TestStore {
	return &TestStore{
		store: make(map[string]CircuitBreaker),
		mu:    &sync.RWMutex{},
		clock: clock.NewSimulatedClock(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)),
	}
}

func (t *TestStore) Lock(_ context.Context, _ string, _ uint64) (*redsync.Mutex, error) {
	return nil, nil
}

func (t *TestStore) Unlock(_ context.Context, _ *redsync.Mutex) error {
	return nil
}

func (t *TestStore) Keys(_ context.Context, s string) (keys []string, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for key := range t.store {
		if strings.HasPrefix(key, s) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

func (t *TestStore) GetOne(_ context.Context, s string) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	res, ok := t.store[s]
	if !ok {
		return "", ErrCircuitBreakerNotFound
	}

	vv := res.String()
	if vv == "" {
		return "", errors.New("an error occurred decoding the circuit breaker")
	}

	return vv, nil
}

func (t *TestStore) GetMany(_ context.Context, keys ...string) (vals []interface{}, err error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, key := range keys {
		if _, ok := t.store[key]; ok {
			vals = append(vals, t.store[key])
		} else {
			vals = append(vals, nil)
		}
	}

	return vals, nil
}

func (t *TestStore) SetOne(_ context.Context, key string, i interface{}, _ time.Duration) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.store[key] = i.(CircuitBreaker)
	return nil
}

func (t *TestStore) SetMany(ctx context.Context, m map[string]CircuitBreaker, duration time.Duration) error {
	for k, v := range m {
		if err := t.SetOne(ctx, k, v, duration); err != nil {
			return err
		}
	}
	return nil
}
