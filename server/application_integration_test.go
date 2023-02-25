//go:build integration
// +build integration

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/jaswdr/faker"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ApplicationIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *ApplicationIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *ApplicationIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	_, s.PersonalAPIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *ApplicationIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *ApplicationIntegrationTestSuite) Test_GetApp_AppNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApp_ValidApplication() {
	appID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), app.Title, dbApp.Title)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApp_ValidApplication_WithPersonalAPIKey() {
	appID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), app.Title, dbApp.Title)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApps_ValidApplications() {
	rand.Seed(time.Now().UnixNano())
	totalApps := rand.Intn(5)
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.DB, s.DefaultProject, totalApps)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalApps), resp.Pagination.Total)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApps_ValidApplications_WithPersonalAPIKey() {
	rand.Seed(time.Now().UnixNano())
	totalApps := rand.Intn(5)
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.DB, s.DefaultProject, totalApps)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalApps), resp.Pagination.Total)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApps_Filters() {
	s.T().Skip("Depends on #637")
}

func (s *ApplicationIntegrationTestSuite) Test_CreateApp() {
	appTitle := fmt.Sprintf("Test-%s", ulid.Make().String())
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, appTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), app.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbApp.Title, appTitle)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppWithPersonalAPIKey() {
	appTitle := fmt.Sprintf("Test-%s", ulid.Make().String())
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultProject.UID)
	body := serialize(`{"name": "%s"}`, appTitle)
	req := createRequest(http.MethodPost, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), app.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbApp.Title, appTitle)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateApp_NoName() {
	appTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, appTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateApp_InvalidRequest() {
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	plainBody := ""
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateApp() {
	title := "random-name"
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := true
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", isDisabled, datastore.InactiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	body := serialize(`{
		"name": "%s",
		"support_email": "%s",
		"is_disabled": %t
	}`, title, supportEmail, false)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), title, dbApp.Title)
	require.Equal(s.T(), supportEmail, dbApp.SupportEmail)
	require.Equal(s.T(), datastore.ActiveEndpointStatus, dbApp.Status)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateApp_WithPersonalAPIKey() {
	title := "random-name"
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := false
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", isDisabled, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	body := serialize(`{
		"name": "%s",
		"support_email": "%s",
		"is_disabled": %t
	}`, title, supportEmail, true)
	req := createRequest(http.MethodPut, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), title, dbApp.Title)
	require.Equal(s.T(), supportEmail, dbApp.SupportEmail)
	require.Equal(s.T(), datastore.InactiveEndpointStatus, dbApp.Status)
}

func (s *ApplicationIntegrationTestSuite) Test_DeleteApp() {
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	_, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *ApplicationIntegrationTestSuite) Test_DeleteApp_WithPersonalAPIKey() {
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultProject.UID, appID)
	req := createRequest(http.MethodDelete, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	_, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppEndpoint() {
	appID := ulid.Make().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultProject.UID, appID)
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

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), secret, dbEndpoint.Secrets[0].Value)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppEndpoint_With_Custom_Authentication() {
	appID := ulid.Make().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultProject.UID, appID)
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

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
	require.Equal(s.T(), secret, dbEndpoint.Secrets[0].Value)
	require.Equal(s.T(), dbEndpoint.Authentication, endpoint.Authentication)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateAppEndpoint_With_Custom_Authentication() {
	appID := ulid.Make().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultProject.UID, appID, endpoint.UID)
	plainBody := fmt.Sprintf(`{
		"url": "%s",
		"description": "default endpoint",
		"authentication": {
			"type": "api_key",
			"api_key": {
				"header_name": "x-api-key",
				"header_value": "testapikey"
			}
		}
	}`, endpointURL)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpointResponse datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpointResponse)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
	require.Equal(s.T(), "x-api-key", dbEndpoint.Authentication.ApiKey.HeaderName)
	require.Equal(s.T(), "testapikey", dbEndpoint.Authentication.ApiKey.HeaderValue)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppEndpoint_TestRedirectToProjectsAPI() {
	appID := ulid.Make().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusTemporaryRedirect

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/applications/%s/endpoints?groupID=%s", appID, s.DefaultProject.UID)
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
	fmt.Println("ff", w.Body.String())
	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppEndpoint_WithPersonalAPIKey() {
	appID := ulid.Make().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultProject.UID, appID)
	plainBody := fmt.Sprintf(`{
		"url": "%s",
		"secret": "%s",
		"description": "default endpoint"
	}`, endpointURL, secret)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateAppEndpoint() {
	appID := ulid.Make().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	rand.Seed(time.Now().UnixNano())
	num := rand.Intn(10) + 1
	eventTypes, _ := json.Marshal(f.Lorem().Words(num))
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultProject.UID, appID, endpoint.UID)
	plainBody := fmt.Sprintf(`{
		"url": "%s",
		"events": %s,
		"description": "default endpoint"
	}`, endpointURL, eventTypes)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var dbEndpoint *datastore.Endpoint
	parseResponse(s.T(), w.Result(), &dbEndpoint)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
}

func (s *ApplicationIntegrationTestSuite) Test_GetAppEndpoint() {
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultProject.UID, appID, endpoint.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp datastore.Endpoint
	parseResponse(s.T(), w.Result(), &resp)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, resp.TargetURL)
}

func (s *ApplicationIntegrationTestSuite) Test_GetAppEndpoints() {
	appID := ulid.Make().String()
	rand.Seed(time.Now().UnixNano())
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)
	endpoint.TargetURL = faker.New().Internet().URL()
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)

	err := endpointRepo.UpdateEndpoint(context.Background(), endpoint, endpoint.ProjectID)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultProject.UID, appID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var dbEndpoints []datastore.DeprecatedEndpoint
	parseResponse(s.T(), w.Result(), &dbEndpoints)

	require.Len(s.T(), dbEndpoints, 1)
}

func (s *ApplicationIntegrationTestSuite) Test_ExpireEndpointSecret() {
	appID := ulid.Make().String()
	f := faker.New()
	secret := f.Lorem().Text(25)
	expiration := 7
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEndpointSecret(s.ConvoyApp.A.DB, app, secret)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s/expire_secret", s.DefaultProject.UID, appID, app.UID)
	plainBody := fmt.Sprintf(`{
		"expiration": %d
	}`, expiration)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.DeprecatedEndpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	app, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), app.Secrets[0].ExpiresAt)
}

func (s *ApplicationIntegrationTestSuite) Test_DeleteAppEndpoint() {
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, appID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultProject.UID, appID, app.UID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	appRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	_, err := appRepo.FindEndpointByID(context.Background(), appID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func TestApplicationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationIntegrationTestSuite))
}
