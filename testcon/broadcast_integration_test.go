//go:build integration
// +build integration

package testcon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/cmd/cli"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	cli2 "github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/services"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type BroadcastIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Q              queue.Queuer
	DefaultUser    *datastore.User
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultSub     *datastore.Subscription
}

func (b *BroadcastIntegrationTestSuite) SetupSuite() {
	q, err := setupTestEnv(b.T())
	require.NoError(b.T(), err)

	t := b.T()
	b.DB = getDB()
	b.Q = q

	user, err := testdb.SeedDefaultUser(b.DB)
	require.NoError(b.T(), err)

	org, err := testdb.SeedDefaultOrganisation(b.DB, user)
	require.NoError(b.T(), err)
	b.DefaultOrg = org

	// Setup Default Project.
	b.DefaultProject, err = testdb.SeedDefaultProject(b.DB, org.UID)
	require.NoError(b.T(), err)

	setupConfig(t, b.DB)
	b.DefaultSub = setupSourceSubscription(t, b.DB, b.DefaultProject)
}

func (b *BroadcastIntegrationTestSuite) SetupTest() {
	startDestination()

	_, c := cli.Build()

	startConvoy(c, []string{"worker"})
	time.Sleep(3 * time.Second)

	startConvoy(c, []string{"ingest"})
	time.Sleep(3 * time.Second)
}

func startDestination() {
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			switch r.Method {
			case "GET":
				for k, v := range r.URL.Query() {
					log.Info(fmt.Sprintf("%s: %s\n", k, v))
				}
				_, _ = w.Write([]byte("Received a GET request\n"))
			case "POST":
				reqBody, err := io.ReadAll(r.Body)
				if err != nil {
					log.Fatal(err)
				}

				log.Printf("Received: %s\n", reqBody)
				_, _ = w.Write([]byte("Received a POST request\n"))
			default:
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(http.StatusText(http.StatusNotImplemented)))
			}
		})
		err := http.ListenAndServe(":8889", nil)
		if err != nil {
			log.Fatal()
		}
	}()
}

func startConvoy(c *cli2.ConvoyCli, args []string) {
	go func() {
		buf := new(bytes.Buffer)
		cmd := c.Cmd()
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(args)
		if err := c.Execute(); err != nil {
			log.Fatal(err)
		}
	}()
}

func (b *BroadcastIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(b.T(), b.DB)
	metrics.Reset()
}

func (b *BroadcastIntegrationTestSuite) Test_BroadcastHappyPath() {
	t := b.T()
	db := b.DB

	const eventThreshold = 10_000
	start := time.Now()
	go publishIntermittently(b.T(), b.Q, b.DefaultSub, b.DefaultProject)

	testTimeout := time.After(3 * time.Minute)
	done := make(chan bool)

	checkDBPeriodically(&done, db, eventThreshold)

	select {
	case <-testTimeout:
		t.Fatal("Test timed out while polling db!")
	case <-done:
	}
	duration := time.Since(start)

	c := countEventDeliveries(db, t)
	fmt.Printf("Total events sent => %d", c.Count)
	assert.True(t, c.Count >= eventThreshold, "events sent must be >= "+strconv.Itoa(eventThreshold))
	fmt.Printf(" in %v seconds (RPS = %v event deliveries/s)\n", duration.Seconds(), float64(c.Count)/duration.Seconds())

	l := getEventDeliveryMetrics(db, t)
	fmt.Printf("Latency: %+v\n", l)

	ru := syscall.Rusage{}
	err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	require.NoError(t, err)
	systemTime := time.Duration(ru.Stime.Usec) * time.Microsecond
	userTime := time.Duration(ru.Utime.Usec) * time.Microsecond
	fmt.Printf("CPU time: system = %+v, user = %+v\n", systemTime, userTime)

	// other info/metrics before shutdown
}

func publishIntermittently(t *testing.T, q queue.Queuer, sub *datastore.Subscription, project *datastore.Project) {
	const events = 20_000
	jobs := make(chan int, events)
	results := make(chan int, events)

	const sourceWorkers = 10
	for w := 1; w <= sourceWorkers; w++ {
		go publish(t, q, jobs, results, w, sub, project)
	}

	for j := 1; j <= events; j++ {
		jobs <- j
	}
	close(jobs)

	for a := 1; a <= events; a++ {
		<-results
	}
}

func publish(t *testing.T, q queue.Queuer, jobs chan int, results chan int, id int, sub *datastore.Subscription, project *datastore.Project) {
	for j := range jobs {
		log.Printf("source %d started  job %d\n", id, j)
		broadcastEvent := &models.BroadcastEvent{
			EventType: "demo.test",
			ProjectID: sub.ProjectID,
			SourceID:  sub.SourceID,
			Data: json.RawMessage(`{
                              "userId": 1,
                              "id": 1,
                              "title": "delectus aut autem",
                              "completed": false
                            }`),
			CustomHeaders:  nil,
			IdempotencyKey: ulid.Make().String(),
			JobID:          ulid.Make().String(),
		}

		cbe := &services.CreateBroadcastEventService{
			Queue:          q,
			BroadcastEvent: broadcastEvent,
			Project:        project,
			JobID:          broadcastEvent.JobID,
		}

		err := cbe.Run(context.Background())
		assert.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
		log.Printf("source %d finished  job %d\n", id, j)
		results <- j * 2
	}
}

func checkDBPeriodically(done *chan bool, db database.Database, threshold int) {
	ticker := time.NewTicker(5 * time.Second)
	quit := make(chan struct{})
	var noResultsCtr atomic.Uint64
	go func() {
		for {
			select {
			case <-ticker.C:
				fmt.Println("Checking db for data...")
				countQ := "select count(*) from convoy.event_deliveries"
				rows, err := db.GetDB().Queryx(countQ)
				if err != nil {
					panic(err)
				}
				for rows.Next() {
					var c counter
					err = rows.StructScan(&c)
					if err != nil {
						panic(err)
					}
					fmt.Println("Count ", c.Count)

					if c.Count > 1000 {
						fmt.Println("Ingest processing successfully")
						noResultsCtr.Store(0) // reset

						if c.Count > threshold {
							fmt.Println("Ingest completed successfully")
							*done <- true
							quit <- struct{}{}
							return
						}
					} else {
						noResultsCtr.Add(1)
						if noResultsCtr.Load() == 1000 {
							fmt.Println("Ingest error - no result after 10 checks")
							*done <- true
							return
						}
					}
				}
				err = rows.Close()
				if err != nil {
					panic(err)
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}

func countEventDeliveries(db database.Database, t *testing.T) counter {
	finalCountQ := "select count(*) from convoy.event_deliveries"
	rows, err := db.GetDB().Queryx(finalCountQ)
	assert.NoError(t, err)
	var c counter
	for rows.Next() {
		err = rows.StructScan(&c)
		assert.NoError(t, err)
	}
	return c
}

type counter = struct {
	Count int
}

func getEventDeliveryMetrics(db database.Database, t *testing.T) latency {
	q := "select min(latency_seconds) as min, max(latency_seconds) as max, avg(latency_seconds) as avg from convoy.event_deliveries where status='Success'"
	rows, err := db.GetDB().Queryx(q)
	assert.NoError(t, err)
	var l latency
	for rows.Next() {
		err = rows.StructScan(&l)
		assert.NoError(t, err)
	}
	return l
}

type latency = struct {
	Min string
	Max string
	Avg string
}

func TestBroadcastIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(BroadcastIntegrationTestSuite))
}
