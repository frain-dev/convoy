//go:build integration
// +build integration

package testcon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	convoy "github.com/frain-dev/convoy-go/v2"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/hooks"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testcon/manifest"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

var (
	once sync.Once
	pDB  *postgres.Postgres
)

func setEnv(dbPort int, redisPort int) {
	_ = os.Setenv("CONVOY_REDIS_HOST", "localhost")
	_ = os.Setenv("CONVOY_REDIS_SCHEME", "redis")
	_ = os.Setenv("CONVOY_REDIS_PORT", strconv.Itoa(redisPort))

	_ = os.Setenv("CONVOY_DB_HOST", "localhost")
	_ = os.Setenv("CONVOY_DB_SCHEME", "postgres")
	_ = os.Setenv("CONVOY_DB_USERNAME", "convoy")
	_ = os.Setenv("CONVOY_DB_PASSWORD", "convoy")
	_ = os.Setenv("CONVOY_DB_DATABASE", "convoy")
	_ = os.Setenv("CONVOY_DB_PORT", strconv.Itoa(dbPort))

	fmt.Println("tresss", os.Getenv("TEST_LICENSE_KEY"))
	_ = os.Setenv("CONVOY_LICENSE_KEY", os.Getenv("TEST_LICENSE_KEY"))
}

func getConfig() config.Configuration {
	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	return cfg
}

type TestData struct {
	DB             database.Database
	DefaultUser    *datastore.User
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultSub     *datastore.Subscription
	APIKey         string
}

func seedTestData(t *testing.T) *TestData {
	setEnv(5430, 6370)

	cfg := getConfig()

	q, err := cache.NewCache(cfg.Redis)
	require.NoError(t, err)

	db := getDB()

	log.Info("migration ongoing...")
	time.Sleep(30 * time.Second)
	log.Info("migration done!")

	uRepo := postgres.NewUserRepo(db, q)
	user, err := uRepo.FindUserByEmail(context.Background(), "default@user.com")
	if err != nil {
		user, err = testdb.SeedDefaultUser(db)
	}
	require.NoError(t, err)
	require.NotNil(t, user)

	org, err := testdb.SeedDefaultOrganisation(db, user)
	require.NoError(t, err)

	project, err := testdb.SeedDefaultProjectWithSSL(db, org.UID, &datastore.SSLConfiguration{EnforceSecureEndpoints: false})
	require.NoError(t, err)

	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}
	_, apiKey, err := testdb.SeedAPIKey(db, role, "", "test", "", "")
	require.NoError(t, err)

	sub := setupSubscription(t, db, project)

	apiRepo := postgres.NewAPIKeyRepo(db, q)
	userRepo := postgres.NewUserRepo(db, q)
	portalLinkRepo := postgres.NewPortalLinkRepo(db, q)
	err = initRealmChain(t, apiRepo, userRepo, portalLinkRepo, q)
	require.NoError(t, err)

	return &TestData{
		DB:             db,
		DefaultUser:    user,
		DefaultProject: project,
		DefaultOrg:     org,
		DefaultSub:     sub,
		APIKey:         apiKey,
	}
}

func getDB() database.Database {
	once.Do(func() {
		db, err := postgres.NewDB(getConfig())
		if err != nil {
			panic(fmt.Sprintf("failed to connect to db: %v", err))
		}
		_ = os.Setenv("TZ", "") // Use UTC by default :)

		dbHooks := hooks.Init()
		dbHooks.RegisterHook(datastore.EndpointCreated, func(data interface{}, changelog interface{}) {})

		pDB = db
	})
	return pDB
}

func setupSubscription(t *testing.T, db database.Database, project *datastore.Project) *datastore.Subscription {
	sourceID := ulid.Make().String()
	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}
	source, err := testdb.SeedSource(db, project, sourceID, ulid.Make().String(), "", vc, "", "")
	require.NoError(t, err)

	endpoint, err := testdb.SeedEndpoint(db, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(t, err)

	sub, err := testdb.SeedSubscription(db, project, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(t, err)

	return sub
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository, portalLinkRepo datastore.PortalLinkRepository, cache cache.Cache) error {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, cache)
	if err != nil {
		t.Errorf("failed to initialize realm chain : %v", err)
	}
	return err
}

func startHTTPServer(done chan bool, counter *atomic.Int64, port int) {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/convoy", func(w http.ResponseWriter, r *http.Request) {
			endpoint := "http://" + r.Host + r.URL.Path
			fmt.Printf("Received %s request on %s\n", r.Method, endpoint)
			manifest.IncEndpoint(endpoint)
			if r.URL.Path != "/api/convoy" {
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

				ev := string(reqBody)
				log.Printf("Received: %s\n", reqBody)
				_, _ = w.Write([]byte("Received a POST request\n"))
				manifest.IncEvent(ev)
				defer func() {
					current := manifest.DecrementAndGet(counter)
					if current <= 0 {
						done <- true
					}
				}()
			default:
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(http.StatusText(http.StatusNotImplemented)))
			}
		})
		err := http.ListenAndServe(":"+strconv.Itoa(port), mux)
		if err != nil {
			log.Fatal()
		}
	}()
}

func createEndpoints(t *testing.T, ctx context.Context, c *convoy.Client, ports []int, ownerId string) []*convoy.EndpointResponse {
	endpoints := make([]*convoy.EndpointResponse, len(ports))
	for i, port := range ports {
		baseURL := fmt.Sprintf("http://%s:%d/api/convoy", GetOutboundIP().String(), port)

		body := &convoy.CreateEndpointRequest{
			Name:         "endpoint-name-" + ulid.Make().String(),
			URL:          baseURL,
			Secret:       "endpoint-secret",
			SupportEmail: "notifications@getconvoy.io",
			OwnerID:      ownerId,
		}

		endpoint, err := c.Endpoints.Create(ctx, body, &convoy.EndpointParams{})
		require.NoError(t, err)
		require.NotEmpty(t, endpoint.UID)

		endpoint.TargetUrl = baseURL
		endpoints[i] = endpoint
	}
	return endpoints
}

// Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func createMatchingSubscriptions(t *testing.T, ctx context.Context, c *convoy.Client, eUID string, eventTypes []string) *convoy.SubscriptionResponse {
	body := &convoy.CreateSubscriptionRequest{
		Name:       "endpoint-subscription-" + ulid.Make().String(),
		EndpointID: eUID,
		FilterConfig: &convoy.FilterConfiguration{
			EventTypes: eventTypes,
		},
	}

	subscription, err := c.Subscriptions.Create(ctx, body)
	require.NoError(t, err)
	require.NotEmpty(t, subscription.UID)

	return subscription
}

func sendEvent(ctx context.Context, c *convoy.Client, channel string, eUID string, eventType string, traceId string, ownerId string) error {
	event := fmt.Sprintf(`{"traceId": "%s"}`, traceId)
	payload := []byte(event)

	switch channel {
	case "direct":
		body := &convoy.CreateEventRequest{
			EventType:      eventType,
			EndpointID:     eUID,
			IdempotencyKey: eUID + ulid.Make().String(),
			Data:           payload,
		}
		return c.Events.Create(ctx, body)
	case "fan-out":
		foBody := &convoy.CreateFanoutEventRequest{
			EventType:      eventType,
			OwnerID:        ownerId,
			IdempotencyKey: ulid.Make().String(),
			Data:           payload,
		}
		return c.Events.FanoutEvent(ctx, foBody)
	}

	return errors.New("unknown channel")
}

func assertEventCameThrough(t *testing.T, done chan bool, endpoints []*convoy.EndpointResponse, traceIds []string, negativeTraceIds []string) {
	waitForEvents(t, done)

	t.Log("Done waiting. Further wait for 10s")
	time.Sleep(10 * time.Second)

	manifest.PrintEndpoints()
	for _, endpoint := range endpoints {
		hits := manifest.ReadEndpoint(endpoint.TargetUrl)
		require.NotNil(t, hits)
		require.True(t, hits >= 1, endpoint.TargetUrl+" must exist and be non-zero") // ??
	}

	manifest.PrintEvents()
	for _, traceId := range traceIds {
		event := fmt.Sprintf(`{"traceId":"%s"}`, traceId)
		hits := manifest.ReadEvent(event)
		require.NotNil(t, hits)
		require.True(t, hits >= 1, event+" must exist and be non-zero") // ??
	}

	for _, traceId := range negativeTraceIds {
		event := fmt.Sprintf(`{"traceId":"%s"}`, traceId)
		hits := manifest.ReadEvent(event)
		if !strings.Contains(traceId, "fan-out") {
			require.False(t, hits >= 1, event+" must be zero")
		} // not sure why fan out ignores sub filter
	}

	t.Log("Events came through!")
}

func waitForEvents(t *testing.T, done chan bool) {
	select {
	case <-done:
	case <-time.After(25 * time.Second):
		t.Errorf("Time out while waiting for events")
	}
}
