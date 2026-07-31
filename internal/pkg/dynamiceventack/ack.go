package dynamiceventack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix = "convoy:dynamic-event:ack:"
	// resultTTL outlives a single waiter so late BLPop still sees the signal.
	resultTTL = 5 * time.Minute
)

var (
	ErrTimeout  = errors.New("timed out waiting for dynamic event resolve")
	ErrNilRedis = errors.New("redis client required for dynamic event sync ack")
)

// Result is published by the match worker and consumed by CreateDynamicEventService.
type Result struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func redisKey(projectID, eventID string) string {
	return keyPrefix + projectID + ":" + eventID
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
