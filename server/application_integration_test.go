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

	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/jaswdr/faker"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/suite"
)

type ApplicationIntegrationTestSuite struct {
	suite.Suite
	DB             cm.Client
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultGroup   *datastore.Group
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

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.Store)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.Store, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, s.DefaultOrg.UID)

	// Seed Auth
	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	_, s.PersonalAPIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
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
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
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
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app AppResponse
	parseResponse(s.T(), w.Result(), &app)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), app.Title, dbApp.Title)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApp_ValidApplication_WithPersonalAPIKey() {
	appID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app AppResponse
	parseResponse(s.T(), w.Result(), &app)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), app.Title, dbApp.Title)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApps_ValidApplications() {
	rand.Seed(time.Now().UnixNano())
	totalApps := rand.Intn(5)
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, s.DefaultGroup, totalApps)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultGroup.UID)
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
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.Store, s.DefaultGroup, totalApps)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultGroup.UID)
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
	appTitle := fmt.Sprintf("Test-%s", uuid.New().String())
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultGroup.UID)
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
	var app AppResponse
	parseResponse(s.T(), w.Result(), &app)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), app.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbApp.Title, appTitle)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppWithPersonalAPIKey() {
	appTitle := fmt.Sprintf("Test-%s", uuid.New().String())
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultGroup.UID)
	body := serialize(`{"name": "%s"}`, appTitle)
	req := createRequest(http.MethodPost, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app AppResponse
	parseResponse(s.T(), w.Result(), &app)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), app.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbApp.Title, appTitle)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateApp_NoName() {
	appTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultGroup.UID)
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

func (s *ApplicationIntegrationTestSuite) Test_CreateApp_NameNotUnique() {
	appTitle := uuid.New().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", appTitle, true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications", s.DefaultGroup.UID)
	plainBody := fmt.Sprintf(`{
		"group_id": "%s",
		"name": "%s"
	}`, s.DefaultGroup.UID, appTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateApp_InvalidRequest() {
	appID := uuid.New().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	plainBody := ""
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateApp_DuplicateNames() {
	appID := uuid.New().String()
	appTitle := "appTitle"
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", appTitle, false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"support_email": "%s"
	}`, appTitle, "10xengineer@getconvoy.io")
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
	isDisabled := randBool()
	appID := uuid.New().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", isDisabled)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	body := serialize(`{
		"name": "%s",
		"support_email": "%s",
		"is_disabled": %t
	}`, title, supportEmail, !isDisabled)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app AppResponse
	parseResponse(s.T(), w.Result(), &app)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), title, dbApp.Title)
	require.Equal(s.T(), supportEmail, dbApp.SupportEmail)
	require.Equal(s.T(), !isDisabled, dbApp.IsDisabled)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateApp_WithPersonalAPIKey() {
	title := "random-name"
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := randBool()
	appID := uuid.New().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", isDisabled)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	body := serialize(`{
		"name": "%s",
		"support_email": "%s",
		"is_disabled": %t
	}`, title, supportEmail, !isDisabled)
	req := createRequest(http.MethodPut, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app AppResponse
	parseResponse(s.T(), w.Result(), &app)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbApp, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), title, dbApp.Title)
	require.Equal(s.T(), supportEmail, dbApp.SupportEmail)
	require.Equal(s.T(), !isDisabled, dbApp.IsDisabled)
}

func (s *ApplicationIntegrationTestSuite) Test_DeleteApp() {
	appID := uuid.New().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	_, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *ApplicationIntegrationTestSuite) Test_DeleteApp_WithPersonalAPIKey() {
	appID := uuid.New().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s", s.DefaultGroup.UID, appID)
	req := createRequest(http.MethodDelete, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	_, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppEndpoint() {
	appID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultGroup.UID, appID)
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

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), secret, dbEndpoint.Secrets[0].Value)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppEndpoint_With_Custom_Authentication() {
	appID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultGroup.UID, appID)
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

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
	require.Equal(s.T(), secret, dbEndpoint.Secrets[0].Value)
	require.Equal(s.T(), dbEndpoint.Authentication, endpoint.Authentication)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateAppEndpoint_With_Custom_Authentication() {
	appID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultGroup.UID, appID, endpoint.UID)
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

	fmt.Println("code", w.Code)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpointResponse datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpointResponse)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
	require.Equal(s.T(), "x-api-key", dbEndpoint.Authentication.ApiKey.HeaderName)
	require.Equal(s.T(), "testapikey", dbEndpoint.Authentication.ApiKey.HeaderValue)

}

func (s *ApplicationIntegrationTestSuite) Test_CreateAppEndpoint_TestRedirectToProjectsAPI() {
	appID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusTemporaryRedirect

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/applications/%s/endpoints?groupID=%s", appID, s.DefaultGroup.UID)
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
	appID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultGroup.UID, appID)
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

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), endpoint.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateAppEndpoint() {
	appID := uuid.New().String()
	f := faker.New()
	endpointURL := f.Internet().URL()
	rand.Seed(time.Now().UnixNano())
	num := rand.Intn(10) + 1
	eventTypes, _ := json.Marshal(f.Lorem().Words(num))
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
	// endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultGroup.UID, appID, endpoint.UID)
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

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, endpointURL)
}

func (s *ApplicationIntegrationTestSuite) Test_GetAppEndpoint() {
	appID := uuid.New().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
	// endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, s.DefaultGroup.UID)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultGroup.UID, appID, endpoint.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp datastore.Endpoint
	parseResponse(s.T(), w.Result(), &resp)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	dbEndpoint, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbEndpoint.TargetURL, resp.TargetURL)
}

func (s *ApplicationIntegrationTestSuite) Test_GetAppEndpoints() {
	appID := uuid.New().String()
	rand.Seed(time.Now().UnixNano())
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints", s.DefaultGroup.UID, appID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var dbEndpoints []Endpoint
	parseResponse(s.T(), w.Result(), &dbEndpoints)

	require.Len(s.T(), dbEndpoints, 1)
}

func (s *ApplicationIntegrationTestSuite) Test_ExpireEndpointSecret() {
	appID := uuid.New().String()
	f := faker.New()
	secret := f.Lorem().Text(25)
	expiration := 7
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)
	_, _ = testdb.SeedEndpointSecret(s.ConvoyApp.A.Store, app, secret)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s/expire_secret", s.DefaultGroup.UID, appID, app.UID)
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
	var endpoint Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	app, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), app.Secrets[0].ExpiresAt)
}

func (s *ApplicationIntegrationTestSuite) Test_DeleteAppEndpoint() {
	appID := uuid.New().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	app, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, appID, "", false)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/applications/%s/endpoints/%s", s.DefaultGroup.UID, appID, app.UID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	appRepo := cm.NewEndpointRepo(s.ConvoyApp.A.Store)
	_, err := appRepo.FindEndpointByID(context.Background(), appID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func TestApplicationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationIntegrationTestSuite))
}
