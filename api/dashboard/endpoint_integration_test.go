//go:build integration
// +build integration

package dashboard

import (
	"context"
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

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/jaswdr/faker"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type EndpointIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *DashboardHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *EndpointIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *EndpointIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	_, s.PersonalAPIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	// Setup Config.
	err = config.LoadConfig("../testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *EndpointIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, appID)
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
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Title, dbEndpoint.Title)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_ValidEndpoint_WithPersonalAPIKey() {
	endpointID := "123456789"
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Title, dbEndpoint.Title)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints() {
	rand.Seed(time.Now().UnixNano())
	totalEndpoints := rand.Intn(5) + 1
	expectedStatusCode := http.StatusOK

	// Just Before.
	err := testdb.SeedMultipleEndpoints(s.ConvoyApp.A.DB, s.DefaultProject, totalEndpoints)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalEndpoints, len(resp.Content.([]interface{})))

}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints_WithPersonalAPIKey() {
	rand.Seed(time.Now().UnixNano())
	totalEndpoints := rand.Intn(5) + 1
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleEndpoints(s.ConvoyApp.A.DB, s.DefaultProject, totalEndpoints)

	// Arrange.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalEndpoints, len(resp.Content.([]interface{})))
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_Filters() {
	s.T().Skip("Depends on #637")
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint() {
	endpointTitle := fmt.Sprintf("Test-%s", ulid.Make().String())
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s"
	}`, endpointTitle, endpointURL)
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

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbEndpoint.Title)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpointWithPersonalAPIKey() {
	endpointTitle := fmt.Sprintf("Test-%s", ulid.Make().String())
	endpointURL := faker.New().Internet().URL()
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	body := serialize(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s"
		}`, endpointTitle, endpointURL)
	req := createRequest(http.MethodPost, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbApp, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbApp.Title)
	require.Equal(s.T(), endpointURL, dbApp.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_NoName() {
	endpointTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
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

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint_InvalidRequest() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusBadRequest

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	plainBody := ""
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
	endpointURL := faker.New().Internet().URL()
	supportEmail := "10xengineer@getconvoy.io"
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s",
		"support_email": "%s"
 	}`, title, endpointURL, supportEmail)
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

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Title)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint_WithPersonalAPIKey() {
	title := "random-name"
	supportEmail := "10xengineer@getconvoy.io"
	isDisabled := randBool()
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted
	endpointURL := faker.New().Internet().URL()

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", isDisabled, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	body := serialize(`{
		"name": "%s",
		"description": "test endpoint",
		"support_email": "%s",
		"url": "%s",
		"is_disabled": %t
	}`, title, supportEmail, endpointURL, !isDisabled)
	req := createRequest(http.MethodPut, url, s.PersonalAPIKey, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	fmt.Println(w.Body.String())

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Title)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), endpointURL, dbEndpoint.TargetURL)
}

func (s *EndpointIntegrationTestSuite) Test_DeleteEndpoint() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	_, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *EndpointIntegrationTestSuite) Test_DeleteEndpoint_WithPersonalAPIKey() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodDelete, url, s.PersonalAPIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	_, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_With_Custom_Authentication() {
	title := "random-name"
	f := faker.New()
	endpointURL := f.Internet().URL()
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints", s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
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
	}`, title, endpointURL, secret)
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

	require.Equal(s.T(), title, endpoint.Title)
	require.Equal(s.T(), endpointURL, endpoint.TargetURL)
	require.Equal(s.T(), datastore.EndpointAuthenticationType("api_key"), endpoint.Authentication.Type)
	require.Equal(s.T(), "x-api-key", endpoint.Authentication.ApiKey.HeaderName)
	require.Equal(s.T(), "testapikey", endpoint.Authentication.ApiKey.HeaderValue)
}

func (s *EndpointIntegrationTestSuite) Test_ExpireEndpointSecret() {
	endpointID := ulid.Make().String()
	f := faker.New()
	secret := f.Lorem().Text(25)
	expiration := 7
	expectedStatusCode := http.StatusOK

	// Just Before.
	e, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEndpointSecret(s.ConvoyApp.A.DB, e, secret)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/expire_secret", s.DefaultProject.UID, endpointID)
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
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	endpoint2, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), endpoint2.Secrets[0].ExpiresAt)
}

func (s *EndpointIntegrationTestSuite) Test_ToggleEndpointStatus_ActiveStatus() {
	endpointId := ulid.Make().String()

	// Just Before
	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointId, "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/toggle_status", s.DefaultProject.UID, endpointId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var endpoint *datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointId, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointId, dbEndpoint.UID)
	require.Equal(s.T(), datastore.InactiveEndpointStatus, dbEndpoint.Status)
}

func (s *EndpointIntegrationTestSuite) Test_ToggleEndpointStatus_InactiveStatus() {
	endpointId := ulid.Make().String()

	// Just Before
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointId, "", "", false, datastore.InactiveEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/toggle_status", s.DefaultProject.UID, endpointId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Assert
	var endpoint *datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointId, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointId, dbEndpoint.UID)
	require.Equal(s.T(), datastore.ActiveEndpointStatus, dbEndpoint.Status)
}

func (s *EndpointIntegrationTestSuite) Test_ToggleEndpointStatus_PendingStatus() {
	endpointId := ulid.Make().String()

	// Just Before
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointId, "", "", false, datastore.PendingEndpointStatus)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/toggle_status", s.DefaultProject.UID, endpointId)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_ToggleEndpointStatus_UnknownStatus() {
	endpointID := ulid.Make().String()

	// Just Before
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.EndpointStatus("abc"))

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/endpoints/%s/toggle_status", s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func TestEndpointIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EndpointIntegrationTestSuite))
}
