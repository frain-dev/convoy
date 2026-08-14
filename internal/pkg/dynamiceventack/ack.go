package dynamiceventack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/cache"
)

const (
	keyPrefix = "convoy:dynamic-event:ack:"
	// resultTTL outlives a single waiter so late BLPop still sees the signal.
	resultTTL         = 5 * time.Minute
	cachePollInterval = 50 * time.Millisecond
)

var (
	ErrTimeout       = errors.New("timed out waiting for dynamic event resolve")
	ErrNilAcker      = errors.New("dynamic event sync ack unavailable")
	ErrNilRedis      = errors.New("redis client required for dynamic event sync ack")
	ErrNilCache      = errors.New("cache required for dynamic event sync ack")
	ErrCacheConsume  = errors.New("cache does not support atomic consume")
	ErrInvalidCached = errors.New("invalid cached dynamic event ack")
)

// Result is published by the match worker and consumed by CreateDynamicEventService.
type Result struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type Acker interface {
	Publish(ctx context.Context, projectID, eventID string, result Result) error
	Wait(ctx context.Context, projectID, eventID string, timeout time.Duration) (Result, error)
}

type redisAcker struct {
	client redis.UniversalClient
}

type cacheAcker struct {
	cache cache.Cache
}

type cacheConsumer interface {
	Consume(ctx context.Context, key string, dest interface{}) (bool, error)
}

type cachedResult struct {
	Published bool
	Result    Result
}

func NewRedisAcker(rdb redis.UniversalClient) Acker {
	return &redisAcker{client: rdb}
}

func NewCacheAcker(brokerCache cache.Cache) Acker {
	return &cacheAcker{cache: brokerCache}
}

func redisKey(projectID, eventID string) string {
	return keyPrefix + projectID + ":" + eventID
}

func (a *redisAcker) Publish(ctx context.Context, projectID, eventID string, result Result) error {
	return Publish(ctx, a.client, projectID, eventID, result)
}

func (a *redisAcker) Wait(ctx context.Context, projectID, eventID string, timeout time.Duration) (Result, error) {
	return Wait(ctx, a.client, projectID, eventID, timeout)
}

func (a *cacheAcker) Publish(ctx context.Context, projectID, eventID string, result Result) error {
	if a.cache == nil {
		return ErrNilCache
	}
	return a.cache.Set(ctx, redisKey(projectID, eventID), cachedResult{Published: true, Result: result}, resultTTL)
}

func (a *cacheAcker) Wait(ctx context.Context, projectID, eventID string, timeout time.Duration) (Result, error) {
	if a.cache == nil {
		return Result{}, ErrNilCache
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	key := redisKey(projectID, eventID)
	consumer, ok := a.cache.(cacheConsumer)
	if !ok {
		return Result{}, ErrCacheConsume
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(cachePollInterval)
	defer ticker.Stop()

	for {
		var cached cachedResult
		found, err := consumer.Consume(ctx, key, &cached)
		if err != nil {
			return Result{}, fmt.Errorf("wait for dynamic event ack: %w", err)
		}
		if found {
			if !cached.Published {
				return Result{}, ErrInvalidCached
			}
			return cached.Result, nil
		}

		select {
		case <-ctx.Done():
			return Result{}, fmt.Errorf("wait for dynamic event ack: %w", ctx.Err())
		case <-timer.C:
			return Result{}, ErrTimeout
		case <-ticker.C:
		}
	}
}

// Publish stores the resolve outcome for a waiting HTTP request.
// Failure policy: callers should log publish errors; the waiter fail-closes on timeout.
func Publish(ctx context.Context, rdb redis.UniversalClient, projectID, eventID string, result Result) error {
	if rdb == nil {
		return ErrNilRedis
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	key := redisKey(projectID, eventID)
	pipe := rdb.Pipeline()
	pipe.RPush(ctx, key, payload)
	pipe.Expire(ctx, key, resultTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// Wait blocks until a result is published or timeout elapses.
// Failure policy: timeout and redis errors fail closed (no 201).
func Wait(ctx context.Context, rdb redis.UniversalClient, projectID, eventID string, timeout time.Duration) (Result, error) {
	if rdb == nil {
		return Result{}, ErrNilRedis
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	key := redisKey(projectID, eventID)
	vals, err := rdb.BLPop(ctx, timeout, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
			return Result{}, ErrTimeout
		}
		// go-redis returns redis.Nil-equivalent via timeout as err with empty vals;
		// BLPop timeout surfaces as redis.Nil.
		if err == redis.Nil {
			return Result{}, ErrTimeout
		}
		return Result{}, fmt.Errorf("wait for dynamic event ack: %w", err)
	}
	if len(vals) < 2 {
		return Result{}, ErrTimeout
	}

	var result Result
	if err := json.Unmarshal([]byte(vals[1]), &result); err != nil {
		return Result{}, fmt.Errorf("decode dynamic event ack: %w", err)
	}
	return result, nil
}
