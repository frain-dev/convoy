package postgres

import (
	"context"
	"time"

	"github.com/lib/pq"
)

const queueNotifyChannel = "convoy_queue_jobs"

const (
	listenerMinReconnect = 10 * time.Second
	listenerMaxReconnect = time.Minute
	listenerPingInterval = 90 * time.Second
	notifyTimeout        = time.Second
)

// Wake returns a channel that receives when pending work may be available.
// Nil when LISTEN/NOTIFY is disabled (no PostgresConnString).
func (q *PostgresQueue) Wake() <-chan struct{} {
	return q.wake
}

func (q *PostgresQueue) signalWake() {
	if q.wake == nil {
		return
	}
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// notifyPending publishes a wakeup after durable pending work. Failure policy:
// fail open — a missed NOTIFY still drains on the poll-idle backstop.
func (q *PostgresQueue) notifyPending() {
	if !q.notifyEnabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), notifyTimeout)
	defer cancel()
	_, _ = q.db.ExecContext(ctx, "SELECT pg_notify($1, '')", queueNotifyChannel)
}

func (q *PostgresQueue) runListener(connString string) {
	defer close(q.listenerDone)

	listener := pq.NewListener(connString, listenerMinReconnect, listenerMaxReconnect, nil)
	if err := listener.Listen(queueNotifyChannel); err != nil {
		return
	}
	defer listener.Close()

	ping := time.NewTicker(listenerPingInterval)
	defer ping.Stop()

	for {
		select {
		case <-q.listenerQuit:
			return
		case <-ping.C:
			_ = listener.Ping()
		case n := <-listener.Notify:
			if n != nil {
				q.signalWake()
			}
		}
	}
}
