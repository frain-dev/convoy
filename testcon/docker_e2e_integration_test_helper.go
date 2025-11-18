//go:build docker_testcon
// +build docker_testcon

package testcon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

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
	"github.com/frain-dev/convoy/internal/pkg/keys"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/testcon/manifest"
)

var (
	once sync.Once
	pDB  *postgres.Postgres
)

func getConfig() config.Configuration {
	err := config.LoadConfig("./testdata/convoy-host.json")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	// Load CA cert for TLS operations
	err = config.LoadCaCert("", "")
	if err != nil {
		log.Fatal(err)
	}

	_ = os.Setenv("CONVOY_LOCAL_ENCRYPTION_KEY", "test-key")
	_ = os.Setenv("CONVOY_DISPATCHER_SKIP_PING_VALIDATION", "true")

	km, err := keys.NewLocalKeyManager()
	if err != nil {
		log.Fatal(err)
	}
	if km.IsSet() {
		if _, err = km.GetCurrentKeyFromCache(); err != nil {
			log.Fatal(err)
		}
	}
	if err = keys.Set(km); err != nil {
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

	cfg := getConfig()

	q, err := cache.NewCache(cfg.Redis)
	require.NoError(t, err)

	db := getDB()

	log.Info("migration ongoing...")
	time.Sleep(30 * time.Second)
	log.Info("migration done!")

	uRepo := postgres.NewUserRepo(db)
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
		Type:    auth.RoleProjectAdmin,
		Project: project.UID,
	}
	_, apiKey, err := testdb.SeedAPIKey(db, role, "", "test", "", "")
	require.NoError(t, err)

	sub := setupSubscription(t, db, project)

	apiRepo := postgres.NewAPIKeyRepo(db)
	userRepo := postgres.NewUserRepo(db)
	portalLinkRepo := postgres.NewPortalLinkRepo(db)
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
		dbHooks.RegisterHook(datastore.EndpointCreated, func(ctx context.Context, data interface{}, changelog interface{}) {})

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

	logger := log.NewLogger(os.Stderr)
	logger.SetLevel(log.FatalLevel)

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, cache, logger)
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
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Received a GET request\n"))
			case "POST":
				reqBody, err := io.ReadAll(r.Body)
				if err != nil {
					log.Fatal(err)
				}

				ev := string(reqBody)
				fmt.Printf("Received %s request on %s Payload: %s\n", r.Method, endpoint, reqBody)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Received a POST request\n"))
				manifest.IncEvent(ev)
				defer func() {
					current := manifest.DecrementAndGet(counter)
					if current <= 0 {
						select {
						case done <- true:
						default:
							// Channel is closed or full, ignore
						}
					}
				}()
			default:
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(http.StatusText(http.StatusNotImplemented)))
			}
		})

		// Start serving - let endpoint creation retry handle server availability
		err := http.ListenAndServe(":"+strconv.Itoa(port), mux)
		if err != nil {
			log.Fatal(err)
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

		// Retry endpoint creation with exponential backoff
		var endpoint *convoy.EndpointResponse
		var err error
		maxRetries := 5
		retryDelay := 100 * time.Millisecond

		for attempt := 0; attempt < maxRetries; attempt++ {
			endpoint, err = c.Endpoints.Create(ctx, body, &convoy.EndpointParams{})
			if err == nil {
				break
			}

			if attempt < maxRetries-1 {
				t.Logf("Endpoint creation attempt %d failed: %v, retrying in %v", attempt+1, err, retryDelay)
				time.Sleep(retryDelay)
				retryDelay *= 2 // Exponential backoff
			}
		}

		require.NoError(t, err, "Failed to create endpoint after %d attempts", maxRetries)
		require.NotEmpty(t, endpoint.UID)

		endpoint.TargetUrl = baseURL
		endpoints[i] = endpoint
	}
	return endpoints
}

// GetOutboundIP Get preferred outbound ip of this machine
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

	manifest.PrintEndpoints()
	for _, endpoint := range endpoints {
		hits := manifest.ReadEndpoint(endpoint.TargetUrl)
		require.NotNil(t, hits)
		require.Equal(t, hits, len(traceIds), endpoint.TargetUrl+" hits must match events sent")
	}

	manifest.PrintEvents()
	for _, traceId := range traceIds {
		event := fmt.Sprintf(`{"traceId":"%s"}`, traceId)
		hits := manifest.ReadEvent(event)
		require.NotNil(t, hits)
		require.Equal(t, hits, len(endpoints), event+" must match number of matched endpoints")
	}

	for _, traceId := range negativeTraceIds {
		event := fmt.Sprintf(`{"traceId":"%s"}`, traceId)
		hits := manifest.ReadEvent(event)
		require.Equal(t, hits, 0, event+" must not exist")
	}

	t.Log("Events came through!")
}

func waitForEvents(t *testing.T, done chan bool) {
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Errorf("Time out while waiting for events")
	}
}

// Global map to track received form data for assertions
var (
	receivedFormData = make(map[string]string)
	formDataMutex    sync.RWMutex
)

func startFormHTTPServer(done chan bool, counter *atomic.Int64, port int) {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/api/convoy", func(w http.ResponseWriter, r *http.Request) {
			endpoint := "http://" + r.Host + r.URL.Path
			manifest.IncEndpoint(endpoint)

			if r.URL.Path != "/api/convoy" {
				http.NotFound(w, r)
				return
			}

			// Set appropriate content type for form data
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")

			switch r.Method {
			case "GET":
				for k, v := range r.URL.Query() {
					log.Info(fmt.Sprintf("Form GET %s: %s\n", k, v))
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Received a form GET request\n"))
			case "POST":
				reqBody, err := io.ReadAll(r.Body)
				if err != nil {
					log.Fatal(err)
				}

				ev := string(reqBody)
				fmt.Printf("Received form %s request on %s Payload: %s\n", r.Method, endpoint, reqBody)
				fmt.Printf("Content-Type header: %s\n", r.Header.Get("Content-Type"))

				// Check if the payload is JSON or form-encoded data
				var jsonData map[string]interface{}

				// Try to parse as JSON first
				if err := json.Unmarshal(reqBody, &jsonData); err == nil {
					// It's JSON data, use it directly
					log.Info("Received JSON payload, using directly")
				} else {
					// Try to parse as form-encoded data
					formData, err := url.ParseQuery(ev)
					if err != nil {
						log.Errorf("Failed to parse payload as JSON or form data: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte("Payload parsing failed\n"))
						return
					}

					// Convert form data to JSON format for assertion
					jsonData = map[string]interface{}{
						"traceId": formData.Get("traceId"),
					}

					// Parse the formData field which contains JSON string
					if formDataStr := formData.Get("formData"); formDataStr != "" {
						var formDataObj map[string]interface{}
						if err := json.Unmarshal([]byte(formDataStr), &formDataObj); err == nil {
							jsonData["formData"] = formDataObj
						} else {
							// Fallback to flat structure if parsing fails
							jsonData["formData"] = map[string]string{
								"name":  formData.Get("name"),
								"email": formData.Get("email"),
							}
						}
					} else {
						// Fallback to flat structure
						jsonData["formData"] = map[string]string{
							"name":  formData.Get("name"),
							"email": formData.Get("email"),
						}
					}
				}

				jsonBytes, err := json.Marshal(jsonData)
				if err != nil {
					log.Errorf("Failed to marshal data to JSON: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("Data conversion failed\n"))
					return
				}

				if !assertFormDataReceived(string(jsonBytes)) {
					log.Error("Form data assertion failed")
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte("Form data validation failed\n"))
					return
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Received a form POST request\n"))

				// Store event in the format expected by assertions (just traceId)
				traceId := jsonData["traceId"]
				if traceId != nil {
					eventForManifest := fmt.Sprintf(`{"traceId":"%s"}`, traceId)
					manifest.IncEvent(eventForManifest)
				} else {
					manifest.IncEvent(ev)
				}
				defer func() {
					current := manifest.DecrementAndGet(counter)
					if current <= 0 {
						select {
						case done <- true:
						default:
							// Channel is closed or full, ignore
						}
					}
				}()
			default:
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = w.Write([]byte(http.StatusText(http.StatusNotImplemented)))
			}
		})

		// Start serving - let endpoint creation retry handle server availability
		err := http.ListenAndServe(":"+strconv.Itoa(port), mux)
		if err != nil {
			log.Fatal(err)
		}
	}()
}

// assertFormDataReceived validates that form data was received correctly
func assertFormDataReceived(formData string) bool {
	// Parse the JSON form data
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(formData), &data); err != nil {
		log.Errorf("Failed to parse form data JSON: %v", err)
		return false
	}

	// Check if traceId exists
	traceId, ok := data["traceId"].(string)
	if !ok {
		log.Error("traceId not found in form data")
		return false
	}

	// Store the received form data for later assertion
	formDataMutex.Lock()
	receivedFormData[traceId] = formData
	formDataMutex.Unlock()

	// Check if formData field exists
	formDataField, ok := data["formData"].(map[string]interface{})
	if !ok {
		log.Error("formData field not found in received data")
		return false
	}

	// Check if user field exists within formData
	userField, ok := formDataField["user"].(map[string]interface{})
	if !ok {
		log.Error("user field not found in formData")
		return false
	}

	// Validate required form fields
	if name, ok := userField["name"].(string); !ok || name == "" {
		log.Error("name field missing or empty in form data")
		return false
	}

	if email, ok := userField["email"].(string); !ok || email == "" {
		log.Error("email field missing or empty in form data")
		return false
	}

	log.Infof("Form data validation passed for traceId: %s", traceId)
	return true
}

// assertFormDataReceivedByEndpoint verifies that form data was received by the endpoint
func assertFormDataReceivedByEndpoint(t *testing.T, traceId, expectedName, expectedEmail string) {
	// Wait a bit for the form data to be processed
	time.Sleep(100 * time.Millisecond)

	formDataMutex.RLock()
	receivedData, exists := receivedFormData[traceId]
	formDataMutex.RUnlock()

	require.True(t, exists, "Form data for traceId %s was not received", traceId)
	require.NotEmpty(t, receivedData, "Received form data is empty for traceId %s", traceId)

	// Parse the received form data
	var data map[string]interface{}
	err := json.Unmarshal([]byte(receivedData), &data)
	require.NoError(t, err, "Failed to parse received form data JSON")

	// Verify traceId matches
	actualTraceId, ok := data["traceId"].(string)
	require.True(t, ok, "traceId field not found in received data")
	require.Equal(t, traceId, actualTraceId, "traceId mismatch")

	// Verify form data structure
	formDataField, ok := data["formData"].(map[string]interface{})
	require.True(t, ok, "formData field not found in received data")

	// Verify user field exists within formData
	userField, ok := formDataField["user"].(map[string]interface{})
	require.True(t, ok, "user field not found in formData")

	// Verify name field
	actualName, ok := userField["name"].(string)
	require.True(t, ok, "name field not found in form data")
	require.Equal(t, expectedName, actualName, "name field mismatch")

	// Verify email field
	actualEmail, ok := userField["email"].(string)
	require.True(t, ok, "email field not found in form data")
	require.Equal(t, expectedEmail, actualEmail, "email field mismatch")

	t.Logf("✅ Form data assertion passed for traceId: %s", traceId)
	t.Logf("   Name: %s", actualName)
	t.Logf("   Email: %s", actualEmail)
}
