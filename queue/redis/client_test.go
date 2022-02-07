//go:build integration
// +build integration

package redis

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/vmihailenco/taskq/v3"
)

func TestWrite(t *testing.T) {
	tests := []struct {
		name            string
		queueName       string
		appID           string
		configFile      string
		eventID         string
		eventDeliveryID string
		eventDelivery   *datastore.EventDelivery
		queueLen        int
	}{
		{
			name:            "Write a single event to queue",
			queueName:       uuid.NewString(),
			appID:           uuid.NewString(),
			configFile:      "../testdata/convoy_redis.json",
			eventID:         uuid.NewString(),
			eventDeliveryID: uuid.NewString(),
			queueLen:        1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eventDelivery := &datastore.EventDelivery{
				UID: tc.eventDeliveryID,
				EventMetadata: &datastore.EventMetadata{
					UID: tc.eventID,
				},
				Status: datastore.SuccessEventStatus,
				AppMetadata: &datastore.AppMetadata{
					UID: tc.appID,
				},
			}
			taskName := convoy.TaskName(uuid.NewString())
			configFile := tc.configFile
			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Write(context.TODO(), taskName, eventDelivery, 0)
			if err != nil {
				t.Fatalf("Failed to write to queue: %v", err)
			}
			queueLength, err := eventQueue.Consumer().Queue().Len()

			if err != nil {
				t.Fatalf("Failed to get queue length: %v", err)
			}
			if queueLength != tc.queueLen {
				t.Fatalf("Length = %q, Want: %v", queueLength, tc.queueLen)

			}

		})
	}

}

func TestConsumer(t *testing.T) {
	tests := []struct {
		name       string
		configFile string
		queueName  string
	}{
		{
			name:       "Start consumer",
			queueName:  uuid.NewString(),
			configFile: "../testdata/convoy_redis.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile

			eventQueue := initializeQueue(configFile, tc.queueName, t)
			err := eventQueue.Consumer().Start(context.TODO())
			if err != nil {
				t.Fatalf("Failed to start consumer: %v", err)
			}
		})
	}
}

func TestCheckEventDeliveryinStream(t *testing.T) {
	tests := []struct {
		name       string
		queueName  string
		configFile string
		start      string
		end        string
		tFN        func(context.Context, *RedisQueue, string, string) (string, error)
		expected   bool
	}{
		{
			name:       "Single EventDelivery in Stream",
			queueName:  "EventQueue",
			configFile: "../testdata/convoy_redis.json",
			start:      "-",
			end:        "+",
			tFN: func(ctx context.Context, q *RedisQueue, start string, end string) (string, error) {
				xmsgs, err := q.XRange(ctx, start, end).Result()
				if err != nil {
					return "", err
				}
				if len(xmsgs) <= 0 {
					return "", nil
				}
				msgs := make([]taskq.Message, len(xmsgs))
				xmsg := &xmsgs[len(xmsgs)-1]
				msg := &msgs[len(msgs)-1]

				err = unmarshalMessage(msg, xmsg)

				if err != nil {
					return "", err
				}

				value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
				return value, nil
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile
			eventQueue := initializeQueue(configFile, tc.queueName, t).(*RedisQueue)
			id, err := tc.tFN(context.Background(), eventQueue, tc.start, tc.end)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}
			if id != "" {
				check, err := eventQueue.CheckEventDeliveryinStream(context.Background(), id, tc.start, tc.end)
				if err != nil {
					t.Fatalf("Check failed with error: %v", err)
				}
				if check != tc.expected {
					t.Fatalf("Check = %q, Want: %v", strconv.FormatBool(check), strconv.FormatBool(tc.expected))

				}
			}
		})
	}
}

func TestCheckEventDeliveryinZSET(t *testing.T) {
	tests := []struct {
		name       string
		queueName  string
		configFile string
		min        string
		max        string
		tFN        func(context.Context, *RedisQueue, string, string) (string, error)
		expected   bool
	}{
		{
			name:       "Single EventDelivery in ZSET",
			queueName:  "EventQueue",
			configFile: "../testdata/convoy_redis.json",
			min:        "-inf",
			max:        "+inf",
			tFN: func(ctx context.Context, q *RedisQueue, min string, max string) (string, error) {
				bodies, err := q.ZRangebyScore(ctx, min, max)

				if err != nil {
					return "", err
				}
				if len(bodies) <= 0 {
					return "", nil
				}
				body := bodies[len(bodies)-1]
				var msg taskq.Message
				err = msg.UnmarshalBinary([]byte(body))

				if err != nil {
					return "", err
				}

				value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])

				return value, nil
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile
			eventQueue := initializeQueue(configFile, tc.queueName, t).(*RedisQueue)
			id, err := tc.tFN(context.Background(), eventQueue, tc.min, tc.max)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}
			if id != "" {
				check, err := eventQueue.CheckEventDeliveryinZSET(context.Background(), id, tc.min, tc.max)
				if err != nil {
					t.Fatalf("Check failed with error: %v", err)
				}
				if check != tc.expected {
					t.Fatalf("Check = %q, Want: %v", strconv.FormatBool(check), strconv.FormatBool(tc.expected))

				}
			}
		})
	}
}

func TestCheckEventDeliveryinPending(t *testing.T) {
	tests := []struct {
		name       string
		queueName  string
		configFile string
		tFN        func(context.Context, *RedisQueue) (string, error)
		expected   bool
	}{
		{
			name:       "Single EventDelivery in Pending",
			queueName:  "EventQueue",
			configFile: "../testdata/convoy_redis.json",
			tFN: func(ctx context.Context, q *RedisQueue) (string, error) {
				pending, err := q.XPending(ctx)
				if err != nil {
					if strings.HasPrefix(err.Error(), "NOGROUP") {
						return "", nil
					}
					return "", err
				}
				if pending.Count <= 0 {
					return "", nil
				}
				var msg taskq.Message
				xmsgInfoID := pending.Higher
				id := xmsgInfoID

				xmsgs, err := q.XRangeN(ctx, id, id, 1).Result()

				if err != nil {
					return "", err
				}

				if len(xmsgs) == 1 {
					err = unmarshalMessage(&msg, &xmsgs[0])
					if err != nil {
						return "", err
					}

					value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
					return value, nil
				}
				return "", nil
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile
			eventQueue := initializeQueue(configFile, tc.queueName, t).(*RedisQueue)
			id, err := tc.tFN(context.Background(), eventQueue)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}
			if id != "" {
				check, err := eventQueue.CheckEventDeliveryinPending(context.Background(), id)
				if err != nil {
					t.Fatalf("Check failed with error: %v", err)
				}
				if check != tc.expected {
					t.Fatalf("Check = %q, Want: %v", strconv.FormatBool(check), strconv.FormatBool(tc.expected))

				}
			}
		})
	}
}

func TestDeleteEventDeliveryFromStream(t *testing.T) {
	tests := []struct {
		name       string
		queueName  string
		configFile string
		start      string
		end        string
		tFN        func(context.Context, *RedisQueue, string, string) (string, error)
		expected   bool
	}{
		{
			name:       "Delete Single EventDelivery from Stream",
			queueName:  "EventQueue",
			configFile: "../testdata/convoy_redis.json",
			start:      "-",
			end:        "+",
			tFN: func(ctx context.Context, q *RedisQueue, start string, end string) (string, error) {
				xmsgs, err := q.XRange(ctx, start, end).Result()
				if err != nil {
					return "", err
				}
				if len(xmsgs) <= 0 {
					return "", nil
				}
				msgs := make([]taskq.Message, len(xmsgs))
				xmsg := &xmsgs[len(xmsgs)-1]
				msg := &msgs[len(msgs)-1]

				err = unmarshalMessage(msg, xmsg)

				if err != nil {
					return "", err
				}

				value := string(msg.ArgsBin[convoy.EventDeliveryIDLength:])
				return value, nil
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			configFile := tc.configFile
			eventQueue := initializeQueue(configFile, tc.queueName, t).(*RedisQueue)
			id, err := tc.tFN(context.Background(), eventQueue, tc.start, tc.end)
			if err != nil {
				t.Fatalf("Error: %v", err)
			}
			if id != "" {
				check, err := eventQueue.DeleteEvenDeliveryfromStream(context.Background(), id)
				if err != nil {
					t.Fatalf("Delete failed with error: %v", err)
				}
				if check {
					check, err = eventQueue.CheckEventDeliveryinStream(context.Background(), id, tc.start, tc.end)
					if err != nil {
						t.Fatalf("Check failed with error: %v", err)
					}
					if check != tc.expected {
						t.Fatalf("Check = %q, Want: %v", strconv.FormatBool(check), strconv.FormatBool(tc.expected))

					}
				}
			}
		})
	}
}

func initializeQueue(configFile string, name string, t *testing.T) queue.Queuer {
	err := config.LoadConfig(configFile, new(config.Configuration))
	if err != nil {
		t.Fatalf("Failed to load config file: %v", err)
	}
	cfg, err := config.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)

	}

	var qFn taskq.Factory
	var rC *redis.Client
	var opts queue.QueueOptions

	rC, qFn, err = NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to load new client: %v", err)
	}
	opts = queue.QueueOptions{
		Name:    name,
		Type:    "redis",
		Redis:   rC,
		Factory: qFn,
	}

	eventQueue := NewQueue(opts)
	return eventQueue
}
