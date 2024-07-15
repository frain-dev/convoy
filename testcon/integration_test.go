package testcon

import (
	"context"
	"fmt"
	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type IntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	DefaultUser    *datastore.User
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultSub     *datastore.Subscription
	APIKey         string
}

func (b *IntegrationTestSuite) SetupSuite() {
	fmt.Println("setting up suite")
	t := b.T()

	compose, err := tc.NewDockerCompose("./testdata/docker-compose-test.yml")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal), "compose.Down()")
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// ignore ryuk error
	_ = compose.WaitForService("postgres", wait.NewLogStrategy("ready").WithStartupTimeout(60*time.Second)).
		WaitForService("redis_server", wait.NewLogStrategy("Ready to accept connections").WithStartupTimeout(10*time.Second)).
		WaitForService("migrate", wait.NewLogStrategy("migration up succeeded").WithStartupTimeout(60*time.Second)).
		Up(ctx, tc.Wait(true))

	setEnv(5430, 6370)
	seed(t, b)
	fmt.Println("suite set up")
}

func seed(t *testing.T, b *IntegrationTestSuite) {
	cfg := getConfig()

	q, err := cache.NewCache(cfg.Redis)
	require.NoError(t, err)

	b.DB = getDB()
	db := b.DB

	log.Info("migration ongoing...")
	time.Sleep(30 * time.Second)
	log.Info("migration done!")

	user, err := testdb.SeedDefaultUser(db)
	require.NoError(t, err)
	require.NotNil(t, user)
	b.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(db, user)
	require.NoError(t, err)
	b.DefaultOrg = org

	b.DefaultProject, err = testdb.SeedDefaultProject(db, org.UID)
	require.NoError(t, err)

	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: b.DefaultProject.UID,
	}
	_, apiKey, err := testdb.SeedAPIKey(db, role, "", "test", "", "")
	require.NoError(t, err)
	b.APIKey = apiKey

	b.DefaultSub = setupSourceSubscription(t, db, b.DefaultProject)

	apiRepo := postgres.NewAPIKeyRepo(db, q)
	userRepo := postgres.NewUserRepo(db, q)
	portalLinkRepo := postgres.NewPortalLinkRepo(db, q)
	err = initRealmChain(t, apiRepo, userRepo, portalLinkRepo, q)
	require.NoError(t, err)
}

func (b *IntegrationTestSuite) SetupTest() {
	fmt.Println("setting up test")
	ctx := context.Background()
	t := b.T()

	projectID := b.DefaultProject.UID
	baseURL := "http://localhost:5015/api/v1/projects/" + projectID + "/events"
	apiKey := b.APIKey

	c := convoy.New(baseURL, apiKey, projectID)

	body := &convoy.CreateEndpointRequest{
		Name:         "endpoint-name",
		URL:          "http://play.getconvoy.io/ingest/DQzxCcNKTB7SGqzm",
		Secret:       "endpoint-secret",
		SupportEmail: "notifications@getconvoy.io",
	}

	endpoint, err := c.Endpoints.Create(ctx, body, &convoy.EndpointParams{})
	require.NoError(t, err)
	log.Printf(endpoint.UID)
}

func (b *IntegrationTestSuite) TearDownTest() {
	fmt.Println("tearing down test")
}

func (b *IntegrationTestSuite) Test_Ingest() {
	t := b.T()
	db := b.DB

	const eventThreshold = 10_000
	start := time.Now()
	//go publishIntermittently(b.DefaultSub, b.DefaultProject)

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
}

//func publish(jobs chan int, results chan int, id int, sub *datastore.Subscription, project *datastore.Project) {
//	for j := range jobs {
//		log.Printf("source %d started  job %d\n", id, j)
//
//		method := "POST"
//
//		payload := strings.NewReader(`{
//            "endpoint_id": "` + sub.EndpointID + `",
//            "event_type": "test.webbook.event.docker",
//            "data": { "Hello": "World", "Test": "Data" }
//            }`)
//
//		client := &http.Client{}
//		req, err := http.NewRequest(method, url, payload)
//
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//		req.Header.Add("Content-Type", "application/json")
//		req.Header.Add("Authorization", "Basic ZGVmYXVsdEB1c2VyLmNvbTpwYXNzd29yZA==")
//
//		res, err := client.Do(req)
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//
//		body, err := io.ReadAll(res.Body)
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//		log.Println(string(body))
//		err = res.Body.Close()
//		if err != nil {
//			fmt.Println(err)
//			return
//		}
//
//		time.Sleep(10 * time.Millisecond)
//		log.Printf("source %d finished  job %d\n", id, j)
//		results <- j * 2
//	}
//}

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

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (b *IntegrationTestSuite) Test_FanOut() {
	t := b.T()
	// do some testing here
	require.Equal(t, 2+2, 4)
}

func (b *IntegrationTestSuite) Test_Dynamic() {
	t := b.T()
	// do some testing here
	require.Equal(t, 2+2, 4)
}
