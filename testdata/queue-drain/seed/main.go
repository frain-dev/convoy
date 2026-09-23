// This helper seeds synthetic accepted work in a disposable application QA stack.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/rdb"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/queue/drain"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
	"github.com/frain-dev/convoy/worker/task"
	"github.com/oklog/ulid/v2"
)

func main() {
	if os.Getenv("QUEUE_DRAIN_DISPOSABLE_QA") != "1" {
		panic("disposable QA opt-in is required")
	}
	var cfg config.Configuration
	data, err := os.ReadFile("/etc/convoy.json")
	must(err)
	must(json.Unmarshal(data, &cfg))
	var fixture struct {
		Project   string            `json:"project"`
		Endpoints map[string]string `json:"endpoints"`
	}
	data, err = os.ReadFile("/qa/fixtures.json")
	must(err)
	must(json.Unmarshal(data, &fixture))

	var q interface {
		queue.Queuer
		queue.Inspector
	}
	if cfg.PreviousQueueProvider == config.PostgresQueueProvider {
		db, err := drain.PostgresExecutorDB(cfg)
		must(err)
		defer db.Close()
		pg, err := pgqueue.NewQueue(queue.QueueOptions{DB: db, Type: "postgres", Names: map[string]int{string(convoy.CreateEventQueue): 1}})
		must(err)
		defer pg.Close()
		q = pg
	} else {
		redisCfg, err := drain.RedisCredentials(cfg.Redis, cfg.Queue.Drain.RedisExecutorUsername, cfg.Queue.Drain.RedisExecutorPassword)
		must(err)
		rd, err := rdb.NewClientFromRedisConfig(redisCfg)
		must(err)
		defer rd.Client().Close()
		q = redisqueue.NewQueue(queue.QueueOptions{RedisClient: rd, Type: "redis", Names: map[string]int{string(convoy.CreateEventQueue): 1}})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	delay := time.Duration(0)
	if os.Getenv("QA_JOB_DELAY") != "" {
		delay, err = time.ParseDuration(os.Getenv("QA_JOB_DELAY"))
		must(err)
	}
	for mode, ep := range fixture.Endpoints {
		for n := 0; n < 3; n++ {
			id := ulid.Make().String()
			jobID := queue.JobId{ProjectID: fixture.Project, ResourceID: id}.SingleJobId()
			payload, _ := json.Marshal(map[string]string{"marker": "source-" + id, "mode": mode})
			event := task.CreateEvent{JobID: jobID, Params: task.CreateEventTaskParams{UID: id, ProjectID: fixture.Project, EndpointID: ep, Data: payload, EventType: "qa.drain", IdempotencyKey: "source-" + id, AcknowledgedAt: time.Now()}}
			bytes, err := msgpack.EncodeMsgPack(event)
			must(err)
			must(q.Write(ctx, convoy.CreateEventProcessor, convoy.CreateEventQueue, &queue.Job{ID: jobID, Payload: bytes, Delay: delay}))
		}
	}
	must(q.PauseQueue(ctx, string(convoy.CreateEventQueue)))
	fmt.Println("Seeded six synthetic accepted events into paused previous queue")
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
