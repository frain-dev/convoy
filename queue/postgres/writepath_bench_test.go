package postgres

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/queue"
)

// TestWritePathThroughput measures enqueue throughput against a real database,
// isolating the batch write from HTTP, k6, and the delivery workers. Set
// PG_WRITEPATH_DSN to run it; it is skipped otherwise.
func TestWritePathThroughput(t *testing.T) {
	dsn := os.Getenv("PG_WRITEPATH_DSN")
	if dsn == "" {
		t.Skip("PG_WRITEPATH_DSN not set")
	}

	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	if err = db.Ping(); err != nil {
		t.Fatal(err)
	}

	const (
		writers  = 200
		perWrite = 100
	)

	q, err := NewQueue(queue.QueueOptions{DB: db, Names: map[string]int{"bench": 1}})
	if err != nil {
		t.Fatal(err)
	}

	// Warm the pool so connection setup does not land inside the timed window.
	for i := 0; i < writers; i++ {
		var one int
		if err = db.Get(&one, "SELECT 1"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec("TRUNCATE convoy.queue_jobs"); err != nil {
		t.Fatal(err)
	}

	payload := make([]byte, 1024)
	var failed atomic.Int64
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWrite; i++ {
				job := &queue.Job{ID: ulid.Make().String(), Payload: payload}
				if err := q.Write(context.Background(), convoy.TaskName("bench"), convoy.QueueName("bench"), job); err != nil {
					failed.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	elapsed := time.Since(start)
	total := writers * perWrite

	var stored int
	if err = db.Get(&stored, "SELECT COUNT(*) FROM convoy.queue_jobs"); err != nil {
		t.Fatal(err)
	}
	if stored != total || failed.Load() != 0 {
		t.Fatalf("wrote %d rows and saw %d failures, want %d rows and 0 failures", stored, failed.Load(), total)
	}

	line := fmt.Sprintf("WRITEPATH jobs=%d elapsed=%s throughput=%.0f/s\n",
		total, elapsed.Round(time.Millisecond), float64(total)/elapsed.Seconds())
	t.Log(line)
	if out := os.Getenv("PG_WRITEPATH_OUT"); out != "" {
		f, ferr := os.OpenFile(out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if ferr != nil {
			t.Fatal(ferr)
		}
		defer f.Close()
		if _, ferr = f.WriteString(line); ferr != nil {
			t.Fatal(ferr)
		}
	}
}
