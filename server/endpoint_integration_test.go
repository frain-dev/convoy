//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/jaswdr/faker"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"
)

type EndpointIntegrationTestSuite struct {
	suite.Suite
	DB           cm.Client
	Router       http.Handler
	ConvoyApp    *ApplicationHandler
	DefaultGroup *datastore.Group
	APIKey       string
}

func (s *EndpointIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *EndpointIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, "")

	// Seed Auth
	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *EndpointIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
	metrics.Reset()
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	endpointID := "123"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", endpointID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", endpointID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbApp.UID)
	require.Equal(s.T(), endpoint.Title, dbApp.Title)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints() {
	rand.Seed(time.Now().UnixNano())
	totalEndpoints := rand.Intn(5)
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, s.DefaultGroup, totalEndpoints)

	// Arrange.
	url := "/api/v1/applications"
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalEndpoints), resp.Pagination.Total)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_Filters() {
	s.T().Skip("Depends on #637")
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint() {
	endpointTitle := fmt.Sprintf("Test-%s", uuid.New().String())
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := "/api/v1/applications"
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, endpointTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbApp.Title, endpointTitle)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_NoName() {
	endpointTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := "/api/v1/applications"
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, endpointTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_NameNotUnique() {
	endpointTitle := uuid.New().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", endpointTitle, true)

	// Arrange Request.
	url := "/api/v1/applications"
	plainBody := fmt.Sprintf(`{
		"group_id": "%s",
		"name": "%s"
	}`, s.DefaultGroup.UID, endpointTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint_InvalidRequest() {
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", endpointID)
	plainBody := ""
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint_DuplicateNames() {
	endpointID := uuid.New().String()
	endpointTitle := "appTitle"
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", endpointTitle, false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", false)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", endpointID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"support_email": "%s"
	}`, endpointTitle, "10xengineer@getconvoy.io")
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint() {
	title := "random-name"
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := randBool()
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", isDisabled)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", endpointID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"support_email": "%s",
		"is_disabled": %t
	}`, title, supportEmail, !isDisabled)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Title)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), !isDisabled, dbEndpoint.IsDisabled)
}

func (s *EndpointIntegrationTestSuite) Test_DeleteEndpoint() {
	endpointID := uuid.New().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", endpointID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	_, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *EndpointIntegrationTestSuite) Test_CreateAppEndpoint() {
	endpointID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/applications/%s/endpoints", endpointID)
	plainBody := fmt.Sprintf(`{
		"url": "%s",
		"secret": "%s",
		"description": "default endpoint"
	}`, endpointURL, secret)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_With_Custom_Authentication() {
	endpointID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, endpointID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/applications/%s/endpoints", endpointID)
	plainBody := fmt.Sprintf(`{
		"url": "%s",
		"secret": "%s",
		"description": "default endpoint",
		"authentication": {
			"type": "api_key",
			"api_key": {
				"header_name": "x-api-key",
				"header_value": "testapikey"
			}
		}
	}`, endpointURL, secret)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
	require.Equal(s.T(), dbEndpoint.Authentication, endpoint.Authentication)
}

// func (s *EndpointIntegrationTestSuite) Test_UpdateAppEndpoint() {
// 	appID := uuid.New().String()
// 	f := faker.New()
// 	endpointURL := f.Internet().URL()
// 	secret := f.Lorem().Text(25)
// 	rand.Seed(time.Now().UnixNano())
// 	num := rand.Intn(10) + 1
// 	eventTypes, _ := json.Marshal(f.Lorem().Words(num))
// 	expectedStatusCode := http.StatusAccepted

// 	// Just Before.
// 	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
// 	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)

// 	// Arrange Request
// 	url := fmt.Sprintf("/api/v1/applications/%s/endpoints/%s", appID, endpoint.UID)
// 	plainBody := fmt.Sprintf(`{
// 		"url": "%s",
// 		"secret": "%s",
// 		"events": %s,
// 		"description": "default endpoint"
// 	}`, endpointURL, secret, eventTypes)
// 	body := strings.NewReader(plainBody)
// 	req := createRequest(http.MethodPut, url, s.APIKey, body)
// 	w := httptest.NewRecorder()

// 	// Act.
// 	s.Router.ServeHTTP(w, req)

// 	// Assert.
// 	require.Equal(s.T(), expectedStatusCode, w.Code)

// 	// Deep Assert.
// 	var dbEndpoint *datastore.Endpoint
// 	parseResponse(s.T(), w.Result(), &dbEndpoint)

// 	appRepo := cm.NewApplicationRepo(s.ConvoyApp.A.Store)
// 	dbEndpoint, err := appRepo.FindApplicationEndpointByID(context.Background(), appID, endpoint.UID)
// 	require.NoError(s.T(), err)
// 	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
// 	require.Equal(s.T(), dbEndpoint.Secret, secret)
// }

// func (s *EndpointIntegrationTestSuite) Test_GetAppEndpoint() {
// 	appID := uuid.New().String()
// 	expectedStatusCode := http.StatusOK

// 	// Just Before.
// 	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
// 	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)

// 	// Arrange Request
// 	url := fmt.Sprintf("/api/v1/applications/%s/endpoints/%s", appID, endpoint.UID)
// 	req := createRequest(http.MethodGet, url, s.APIKey, nil)
// 	w := httptest.NewRecorder()

// 	// Act.
// 	s.Router.ServeHTTP(w, req)

// 	// Assert.
// 	require.Equal(s.T(), expectedStatusCode, w.Code)

// 	// Deep Assert.
// 	var resp datastore.Endpoint
// 	parseResponse(s.T(), w.Result(), &resp)

// 	appRepo := cm.NewApplicationRepo(s.ConvoyApp.A.Store)
// 	dbEndpoint, err := appRepo.FindApplicationEndpointByID(context.Background(), appID, endpoint.UID)
// 	require.NoError(s.T(), err)
// 	require.Equal(s.T(), dbEndpoint.TargetURL, resp.TargetURL)
// 	require.Equal(s.T(), dbEndpoint.Secret, resp.Secret)
// }

// func (s *EndpointIntegrationTestSuite) Test_GetAppEndpoints() {
// 	appID := uuid.New().String()
// 	rand.Seed(time.Now().UnixNano())
// 	num := rand.Intn(10)
// 	expectedStatusCode := http.StatusOK

// 	// Just Before.
// 	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
// 	endpoints, _ := testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID, []string{"*"}, num)

// 	// Arrange Request
// 	url := fmt.Sprintf("/api/v1/applications/%s/endpoints", appID)
// 	req := createRequest(http.MethodGet, url, s.APIKey, nil)
// 	w := httptest.NewRecorder()

// 	// Act.
// 	s.Router.ServeHTTP(w, req)

// 	// Assert.
// 	require.Equal(s.T(), expectedStatusCode, w.Code)

// 	// Deep Assert.
// 	var dbEndpoints []datastore.Endpoint
// 	parseResponse(s.T(), w.Result(), &dbEndpoints)

// 	require.Len(s.T(), dbEndpoints, len(endpoints))
// }

// func (s *EndpointIntegrationTestSuite) Test_DeleteAppEndpoint() {
// 	appID := uuid.New().String()
// 	expectedStatusCode := http.StatusOK

// 	// Just Before.
// 	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
// 	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)

// 	// Arrange Request.
// 	url := fmt.Sprintf("/api/v1/applications/%s/endpoints/%s", appID, endpoint.UID)
// 	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
// 	w := httptest.NewRecorder()

// 	// Act.
// 	s.Router.ServeHTTP(w, req)

// 	// Assert.
// 	require.Equal(s.T(), expectedStatusCode, w.Code)

// 	// Deep Assert.
// 	appRepo := cm.NewApplicationRepo(s.ConvoyApp.A.Store)
// 	_, err := appRepo.FindApplicationEndpointByID(context.Background(), appID, endpoint.UID)
// 	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
// }

func TestEndpointIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EndpointIntegrationTestSuite))
}
