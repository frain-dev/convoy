package e2e

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	convoypkg "github.com/frain-dev/convoy"
	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	pgqueue "github.com/frain-dev/convoy/queue/postgres"
)

// TestE2E_PostgresQueue_DeliversAndDrainsAcrossBatches runs the whole delivery
// path on the Postgres queue provider instead of Redis.
//
// The provider ships as a supported option, but every e2e test until now ran
// the Redis leg, so nothing covered enqueue, claim, lease and completion
// against convoy.queue_jobs through a real server and worker. A break there
// looks like "events never deliver", which no unit test on the queue package
// alone would attribute to the wiring.
func TestE2E_PostgresQueue_DeliversAndDrainsAcrossBatches(t *testing.T) {
	env := SetupE2E(t, func(cfg *config.Configuration) {
		// broker.NewTest adds this flag for the postgres provider when it
		// builds the queue; set it here too so the config the server and
		// worker publish agrees with the broker that was built.
		cfg.QueueProvider = config.PostgresQueueProvider
		cfg.EnableFeatureFlag = append(cfg.EnableFeatureFlag, string(fflag.PostgresQueue))
	})
	ctx := context.Background()

	// Proof the provider under test is the one running. Successful jobs are
	// deleted from convoy.queue_jobs, so a row count cannot answer this after
	// the queue drains.
	_, ok := env.App.Queue.(*pgqueue.PostgresQueue)
	require.True(t, ok, "harness built a %T, so this test would have covered redis again", env.App.Queue)

	manifest := NewEventManifest()
	done := make(chan bool, 1)
	var counter atomic.Int64
	port := 19914
	StartMockWebhookServer(t, manifest, done, &counter, port)

	client := convoy.New(env.ServerURL+"/api/v1", env.APIKey, env.Project.UID)
	endpoint := CreateEndpointViaSDK(t, client, port, "pgqueue-"+ulid.Make().String())
	CreateSubscriptionViaSDK(t, client, endpoint.UID, []string{"*"})

	const eventType = "pgqueue.delivery"
	// counter counts down: the mock server decrements per webhook and signals
	// done at zero.
	counter.Store(1)
	SendEventViaSDK(t, client, endpoint.UID, eventType, "trace-"+ulid.Make().String())
	WaitForWebhooks(t, done, 90*time.Second)
	waitForDeliveryCount(t, env, env.Project.UID, 1, 30*time.Second)
	waitForQueueDrain(t, env, 60*time.Second)

	// A second batch runs against a queue table that already holds rows from
	// the first, which is every minute of a real instance after the first one.
	// Claim ordering and lease reclaim only have prior state to trip over here.
	counter.Store(3)
	for i := 0; i < 3; i++ {
		SendEventViaSDK(t, client, endpoint.UID, eventType, "trace-"+ulid.Make().String())
	}
	WaitForWebhooks(t, done, 90*time.Second)
	waitForDeliveryCount(t, env, env.Project.UID, 4, 30*time.Second)
	waitForQueueDrain(t, env, 60*time.Second)

	// Every event reached the endpoint, so none was dropped between the queue
	// and the wire.
	require.Equal(t, 4, manifest.ReadEndpoint(fmt.Sprintf("http://localhost:%d/webhook", port)))

	// Archived means the job exhausted its retries. A delivery to a mock that
	// answers 200 must not land there.
	var archived int
	require.NoError(t, env.App.DB.GetDB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM convoy.queue_jobs WHERE status = 'archived'`).Scan(&archived))
	require.Zero(t, archived, "a delivery job was archived after exhausting retries")
}

// waitForQueueDrain polls until no job is pending or in flight. Asserting on a
// delivery count alone would pass while a job sat claimed by a dead consumer.
func waitForQueueDrain(t *testing.T, env *E2ETestEnv, timeout time.Duration) {
	t.Helper()

	var pending int
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// The schedule queue always holds pending cron rows for the next tick,
		// so a drain that counted them would never settle.
		require.NoError(t, env.App.DB.GetDB().QueryRowContext(context.Background(), `
			SELECT COUNT(*) FROM convoy.queue_jobs
			WHERE status IN ('pending', 'processing')
			  AND queue_name <> $1`, string(convoypkg.ScheduleQueue)).Scan(&pending))
		if pending == 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.Zero(t, pending, "queue did not drain within %v", timeout)
}
