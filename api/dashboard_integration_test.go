//go:build integration
// +build integration

package api

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

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/jaswdr/faker"
	"github.com/oklog/ulid/v2"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type pagedResponse struct {
	Content    interface{}               `json:"content,omitempty"`
	Pagination *datastore.PaginationData `json:"pagination,omitempty"`
}

type AuthIntegrationTestSuite struct {
	suite.Suite
	DB        database.Database
	Router    http.Handler
	ConvoyApp *ApplicationHandler
	jwt       *jwt.Jwt
}

func (u *AuthIntegrationTestSuite) SetupSuite() {
	u.DB = getDB()
	u.ConvoyApp = buildServer()
	u.Router = u.ConvoyApp.BuildControlPlaneRoutes()
}

func (u *AuthIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(u.T(), u.DB)

	err := config.LoadConfig("./testdata/Auth_Config/jwt-convoy.json")
	require.NoError(u.T(), err)

	configuration, err := config.Get()
	require.NoError(u.T(), err)

	u.jwt = jwt.NewJwt(&configuration.Auth.Jwt, u.ConvoyApp.A.Cache)

	apiRepo := postgres.NewAPIKeyRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	initRealmChain(u.T(), apiRepo, userRepo, portalLinkRepo, u.ConvoyApp.A.Cache)
}

func (u *AuthIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(u.T(), u.DB)
	metrics.Reset()
}

func (u *AuthIntegrationTestSuite) Test_LoginUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	// Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
		"username": "%s",
		"password": "%s"
	}`, user.Email, password)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response models.LoginUserResponse
	parseResponse(u.T(), w.Result(), &response)

	require.NotEmpty(u.T(), response.UID)
	require.NotEmpty(u.T(), response.Token.AccessToken)
	require.NotEmpty(u.T(), response.Token.RefreshToken)

	require.Equal(u.T(), user.UID, response.UID)
	require.Equal(u.T(), user.FirstName, response.FirstName)
	require.Equal(u.T(), user.LastName, response.LastName)
	require.Equal(u.T(), user.Email, response.Email)
}

func (u *AuthIntegrationTestSuite) Test_IsSignupEnabled_False() {
	err := config.LoadConfig("./testdata/Auth_Config/jwt-convoy-signup-disabled.json")
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/configuration/is_signup_enabled"
	req := createRequest(http.MethodGet, url, "", nil)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response bool
	parseResponse(u.T(), w.Result(), &response)

	require.Equal(u.T(), false, response)
}

func (u *AuthIntegrationTestSuite) Test_IsSignupEnabled_True() {
	err := config.LoadConfig("./testdata/Auth_Config/jwt-convoy-signup-enabled.json")
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/configuration/is_signup_enabled"
	req := createRequest(http.MethodGet, url, "", nil)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response bool
	parseResponse(u.T(), w.Result(), &response)

	require.Equal(u.T(), true, response)
}

func (u *AuthIntegrationTestSuite) Test_LoginUser_Invalid_Username() {
	// Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
			"username": "%s",
			"password": "%s"
		}`, "random@test.com", "123456")

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusForbidden, w.Code)
}

func (u *AuthIntegrationTestSuite) Test_LoginUser_Invalid_Password() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	// Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
			"username": "%s",
			"password": "%s"
		}`, user.Email, "12345")

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusForbidden, w.Code)
}

func (u *AuthIntegrationTestSuite) Test_RefreshToken() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, token.AccessToken, token.RefreshToken)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response jwt.Token
	parseResponse(u.T(), w.Result(), &response)

	require.NotEmpty(u.T(), response.AccessToken)
	require.NotEmpty(u.T(), response.RefreshToken)
}

func (u *AuthIntegrationTestSuite) Test_RefreshToken_Invalid_Access_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, ulid.Make().String(), token.RefreshToken)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *AuthIntegrationTestSuite) Test_RefreshToken_Invalid_Refresh_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, token.AccessToken, ulid.Make().String())

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *AuthIntegrationTestSuite) Test_LogoutUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/logout"
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)
}

func (u *AuthIntegrationTestSuite) Test_LogoutUser_Invalid_Access_Token() {
	// Arrange Request
	url := "/ui/auth/logout"
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", ulid.Make().String()))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func TestAuthIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(AuthIntegrationTestSuite))
}

type DashboardIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultUser     *datastore.User
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
}

func (s *DashboardIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *DashboardIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	// Setup Default User
	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	// Setup Default Organisation
	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *DashboardIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *DashboardIntegrationTestSuite) TestGetDashboardSummary() {
	ctx := context.Background()
	endpoint := &datastore.Endpoint{
		UID:          "abc",
		ProjectID:    s.DefaultProject.UID,
		Name:         "test-app",
		Secrets:      datastore.Secrets{},
		SupportEmail: "test@suport.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	err := endpointRepo.CreateEndpoint(ctx, endpoint, endpoint.ProjectID)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	sub, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.IncomingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	eventDeliveries := []datastore.EventDelivery{
		{
			UID:            ulid.Make().String(),
			ProjectID:      s.DefaultProject.UID,
			EndpointID:     endpoint.UID,
			EventID:        event.UID,
			SubscriptionID: sub.UID,
			Metadata:       &datastore.Metadata{},
			CreatedAt:      time.Date(2021, time.January, 1, 1, 1, 1, 0, time.UTC),
			UpdatedAt:      time.Date(2021, time.January, 1, 1, 1, 1, 0, time.UTC),
		},
		{
			UID:            ulid.Make().String(),
			ProjectID:      s.DefaultProject.UID,
			EventID:        event.UID,
			SubscriptionID: sub.UID,
			Metadata:       &datastore.Metadata{},
			CreatedAt:      time.Date(2021, time.January, 10, 1, 1, 1, 0, time.UTC),
			UpdatedAt:      time.Date(2021, time.January, 10, 1, 1, 1, 0, time.UTC),
		},
		{
			UID:            ulid.Make().String(),
			ProjectID:      s.DefaultProject.UID,
			EventID:        event.UID,
			SubscriptionID: sub.UID,
			Metadata:       &datastore.Metadata{},
			CreatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
			UpdatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
		},
		{
			UID:            ulid.Make().String(),
			ProjectID:      s.DefaultProject.UID,
			EventID:        event.UID,
			SubscriptionID: sub.UID,
			Metadata:       &datastore.Metadata{},
			CreatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
			UpdatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
		},
		{
			UID:            ulid.Make().String(),
			ProjectID:      s.DefaultProject.UID,
			EventID:        event.UID,
			SubscriptionID: sub.UID,
			Metadata:       &datastore.Metadata{},
			CreatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
			UpdatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
		},
		{
			UID:            ulid.Make().String(),
			ProjectID:      s.DefaultProject.UID,
			EventID:        event.UID,
			SubscriptionID: sub.UID,
			Metadata:       &datastore.Metadata{},
			CreatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
			UpdatedAt:      time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC),
		},
	}

	eventDelivery := postgres.NewEventDeliveryRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	for i := range eventDeliveries {
		err = eventDelivery.CreateEventDelivery(ctx, &eventDeliveries[i])
		require.NoError(s.T(), err)
		_, err = s.DB.GetDB().ExecContext(context.Background(), "UPDATE convoy.event_deliveries SET created_at=$1,updated_at=$2 WHERE id=$3",
			eventDeliveries[i].CreatedAt, eventDeliveries[i].UpdatedAt, eventDeliveries[i].UID)
		require.NoError(s.T(), err)
	}

	type urlQuery struct {
		projectID string
		startDate string
		endDate   string
		Type      string
	}

	tests := []struct {
		name       string
		method     string
		urlQuery   urlQuery
		statusCode int
	}{
		{
			name:       "should_fetch_yearly_dashboard_summary",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				startDate: "2021-01-01T00:00:00",
				endDate:   "2021-01-30T00:00:00",
				Type:      "yearly",
			},
		},
		{
			name:       "should_fetch_monthly_dashboard_summary",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				startDate: "2021-01-01T00:00:00",
				endDate:   "2022-12-27T00:00:00",
				Type:      "monthly",
			},
		},
		{
			name:       "should_fetch_weekly_dashboard_summary",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				startDate: "2021-01-01T00:00:00",
				endDate:   "2022-12-27T00:00:00",
				Type:      "weekly",
			},
		},
		{
			name:       "should_fetch_daily_dashboard_summary",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				startDate: "2021-01-01T00:00:00",
				endDate:   "2022-12-27T00:00:00",
				Type:      "daily",
			},
		},
		{
			name:       "should_error_for_empty_startDate",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				endDate:   "2022-12-27T00:00:00",
				Type:      "daily",
			},
		},
		{
			name:       "should_error_for_invalid_startDate",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				startDate: "2021-01-01",
				endDate:   "2022-12-27T00:00:00",
				Type:      "daily",
			},
		},
		{
			name:       "should_error_for_invalid_type",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				startDate: "2021-01-01T00:00:00",
				endDate:   "2022-12-27T00:00:00",
				Type:      "abc",
			},
		},
		{
			name:       "should_error_for_startDate_greater_than_endDate",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			urlQuery: urlQuery{
				projectID: s.DefaultProject.UID,
				startDate: "2021-01-01T00:00:00",
				endDate:   "2020-12-27T00:00:00",
				Type:      "daily",
			},
		},
	}
	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
			userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
			portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
			initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)

			req := httptest.NewRequest(tc.method, fmt.Sprintf("/ui/organisations/%s/projects/%s/dashboard/summary?startDate=%s&endDate=%s&type=%s", s.DefaultOrg.UID, tc.urlQuery.projectID, tc.urlQuery.startDate, tc.urlQuery.endDate, tc.urlQuery.Type), nil)
			err = s.AuthenticatorFn(req, s.Router)
			require.NoError(s.T(), err)

			w := httptest.NewRecorder()

			s.Router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}

func TestDashboardIntegrationTestSuiteTest(t *testing.T) {
	suite.Run(t, new(DashboardIntegrationTestSuite))
}

func verifyMatch(t *testing.T, w httptest.ResponseRecorder) {
	g := goldie.New(
		t,
		goldie.WithFixtureDir("./testdata"),
		goldie.WithDiffEngine(goldie.ColoredDiff),
	)
	g.Assert(t, t.Name(), w.Body.Bytes())
}

type EndpointIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *EndpointIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *EndpointIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *EndpointIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoint_EndpointNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, appID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

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
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), endpoint.Name, dbEndpoint.Name)
}

func (s *EndpointIntegrationTestSuite) Test_GetEndpoints_ValidEndpoints() {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	totalEndpoints := rnd.Intn(5) + 1
	expectedStatusCode := http.StatusOK

	// Just Before.
	err := testdb.SeedMultipleEndpoints(s.ConvoyApp.A.DB, s.DefaultProject, totalEndpoints)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

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
	endpointURL := "https://www.google.com/webhp"
	expectedStatusCode := http.StatusCreated

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s"
	}`, endpointTitle, endpointURL)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpoint.UID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointTitle, dbEndpoint.Name)
	require.Equal(s.T(), endpointURL, dbEndpoint.Url)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_NoName() {
	endpointTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, endpointTitle)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

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
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpointID)
	plainBody := ""
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EndpointIntegrationTestSuite) Test_UpdateEndpoint() {
	title := "random-name"
	endpointURL := "https://www.google.com/webhp"
	supportEmail := "10xengineer@getconvoy.io"
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpointID)
	plainBody := fmt.Sprintf(`{
		"name": "%s",
		"description": "test endpoint",
		"url": "%s",
		"support_email": "%s"
 	}`, title, endpointURL, supportEmail)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpoint.UID, dbEndpoint.UID)
	require.Equal(s.T(), title, dbEndpoint.Name)
	require.Equal(s.T(), supportEmail, dbEndpoint.SupportEmail)
	require.Equal(s.T(), endpointURL, dbEndpoint.Url)
}

func (s *EndpointIntegrationTestSuite) Test_DeleteEndpoint() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpointID)
	req := createRequest(http.MethodDelete, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	_, err = endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.Error(s.T(), err, datastore.ErrEndpointNotFound)
}

func (s *EndpointIntegrationTestSuite) Test_CreateEndpoint_With_Custom_Authentication() {
	title := "random-name"
	f := faker.New()
	endpointURL := "https://www.google.com/webhp"
	secret := f.Lorem().Text(25)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
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
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	require.Equal(s.T(), title, endpoint.Name)
	require.Equal(s.T(), endpointURL, endpoint.Url)
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
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s/expire_secret", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpointID)
	plainBody := fmt.Sprintf(`{
		"expiration": %d
	}`, expiration)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPut, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var endpoint datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	endpoint2, err := endpointRepo.FindEndpointByID(context.Background(), endpointID, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), endpoint2.Secrets[0].ExpiresAt)
}

func (s *EndpointIntegrationTestSuite) Test_PauseEndpoint_PausedStatus() {
	endpointId := ulid.Make().String()

	// Just Before
	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointId, "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s/pause", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpointId)
	req := createRequest(http.MethodPut, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var endpoint *datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointId, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointId, dbEndpoint.UID)
	require.Equal(s.T(), datastore.PausedEndpointStatus, dbEndpoint.Status)
}

func (s *EndpointIntegrationTestSuite) Test_PauseEndpoint_ActiveStatus() {
	endpointId := ulid.Make().String()

	// Just Before
	_, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointId, "", "", false, datastore.PausedEndpointStatus)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/endpoints/%s/pause", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpointId)
	req := createRequest(http.MethodPut, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var endpoint *datastore.Endpoint
	parseResponse(s.T(), w.Result(), &endpoint)

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbEndpoint, err := endpointRepo.FindEndpointByID(context.Background(), endpointId, s.DefaultProject.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), endpointId, dbEndpoint.UID)
	require.Equal(s.T(), datastore.ActiveEndpointStatus, dbEndpoint.Status)
}

func TestEndpointIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EndpointIntegrationTestSuite))
}

type EventIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *EventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *EventIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *EventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *EventIntegrationTestSuite) Test_CreateEndpointEvent() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", false, datastore.ActiveEndpointStatus)

	bodyStr := `{"endpoint_id": "%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/events", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_CreateEndpointEvent_With_App_ID_Valid_Event() {
	endpointID := ulid.Make().String()
	appID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	// Create an Endpoint with an app ID
	endpoint := &datastore.Endpoint{
		UID:       endpointID,
		Name:      fmt.Sprintf("TestEndpoint-%s", endpointID),
		ProjectID: s.DefaultProject.UID,
		AppID:     appID,
		Secrets: datastore.Secrets{
			{UID: ulid.Make().String()},
		},
		Status: datastore.ActiveEndpointStatus,
	}

	err := postgres.NewEndpointRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache).CreateEndpoint(context.TODO(), endpoint, s.DefaultProject.UID)
	require.NoError(s.T(), err)

	bodyStr := `{"app_id":"%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, appID)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/events", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, "", body)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_CreateEndpointEvent_Endpoint_is_disabled() {
	endpointID := ulid.Make().String()
	expectedStatusCode := http.StatusCreated

	// Just Before.
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, endpointID, "", "", true, datastore.ActiveEndpointStatus)

	bodyStr := `{"endpoint_id": "%s", "event_type":"*", "data":{"level":"test"}}`
	body := serialize(bodyStr, endpointID)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/events", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/events/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvent datastore.Event
	parseResponse(s.T(), w.Result(), &respEvent)
	require.Equal(s.T(), event.UID, respEvent.UID)
}

func (s *EventIntegrationTestSuite) Test_ReplayEndpointEvent_Valid_Event() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, eventID, "*", "", []byte(`{}`))

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/events/%s/replay", s.DefaultProject.OrganisationID, s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodPut, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEndpointEvent_Event_not_found() {
	eventID := ulid.Make().String()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/events/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, eventID)
	req := createRequest(http.MethodGet, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_GetEventDelivery_Valid_EventDelivery() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/eventdeliveries/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, eventDeliveryID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEventDelivery datastore.EventDelivery
	parseResponse(s.T(), w.Result(), &respEventDelivery)
	require.Equal(s.T(), eventDelivery.UID, respEventDelivery.UID)
}

func (s *EventIntegrationTestSuite) Test_GetEventDelivery_Event_not_found() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/eventdeliveries/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, eventDeliveryID)
	req := createRequest(http.MethodGet, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_ResendEventDelivery_Valid_Resend() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	eventDelivery, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/eventdeliveries/%s/resend", s.DefaultProject.OrganisationID, s.DefaultProject.UID, eventDeliveryID)
	req := createRequest(http.MethodPut, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEventDelivery datastore.EventDelivery
	parseResponse(s.T(), w.Result(), &respEventDelivery)
	require.Equal(s.T(), datastore.ScheduledEventStatus, respEventDelivery.Status)
	require.Equal(s.T(), eventDelivery.UID, respEventDelivery.UID)
}

func (s *EventIntegrationTestSuite) Test_BatchRetryEventDelivery_Valid_EventDeliveries() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/eventdeliveries/batchretry?endpointId=%s&eventId=%s&status=%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpoint.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodPost, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *EventIntegrationTestSuite) Test_CountAffectedEventDeliveries_Valid_Filters() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/eventdeliveries/countbatchretryevents?endpointId=%s&eventId=%s&status=%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpoint.UID, event.UID, datastore.FailureEventStatus)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var num struct {
		Num int `json:"num"`
	}
	parseResponse(s.T(), w.Result(), &num)
	require.Equal(s.T(), 3, num.Num)
}

func (s *EventIntegrationTestSuite) Test_ForceResendEventDeliveries_Valid_EventDeliveries() {
	expectedStatusCode := http.StatusOK
	expectedMessage := "3 successful, 0 failed"

	// Just Before.
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	event, _ := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	e1, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)
	require.NoError(s.T(), err)

	e2, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)
	e3, _ := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event, endpoint, s.DefaultProject.UID, ulid.Make().String(), datastore.SuccessEventStatus, subscription)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/eventdeliveries/forceresend", s.DefaultProject.OrganisationID, s.DefaultProject.UID)

	bodyStr := `{"ids":["%s", "%s", "%s"]}`
	body := serialize(bodyStr, e1.UID, e2.UID, e3.UID)

	req := createRequest(http.MethodPost, url, "", body)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(s.T(), err)

	require.Equal(s.T(), expectedMessage, response["message"].(string))
}

func (s *EventIntegrationTestSuite) Test_GetEventsPaged() {
	eventID := ulid.Make().String()
	sourceID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	vc := &datastore.VerifierConfig{
		Type: datastore.BasicAuthVerifier,
		BasicAuth: &datastore.BasicAuth{
			UserName: "Convoy",
			Password: "Convoy",
		},
	}

	_, err = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, ulid.Make().String(), "", vc, "", "")
	require.NoError(s.T(), err)

	e1, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, eventID, "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	e2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	_, err = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", sourceID, []byte(`{}`))
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/events?endpointId=%s&sourceId=%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpoint1.UID, sourceID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.Event
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(respEvents))

	v := []string{e1.UID, e2.UID}
	for i := range respEvents {
		require.Contains(s.T(), v, respEvents[i].UID)
	}
}

func (s *EventIntegrationTestSuite) Test_GetEventDeliveriesPaged() {
	eventDeliveryID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	endpoint1, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	subscription, err := testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, &datastore.Source{}, endpoint1, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, &datastore.FilterConfiguration{
		EventTypes: []string{"*"},
		Filter:     datastore.FilterSchema{Headers: datastore.M{}, Body: datastore.M{}},
	})
	require.NoError(s.T(), err)

	event1, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint1, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	d1, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, eventDeliveryID, datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	d2, err := testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event1, endpoint1, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	endpoint2, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	event2, err := testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint2, s.DefaultProject.UID, ulid.Make().String(), "*", "", []byte(`{}`))
	require.NoError(s.T(), err)

	_, err = testdb.SeedEventDelivery(s.ConvoyApp.A.DB, event2, endpoint2, s.DefaultProject.UID, ulid.Make().String(), datastore.FailureEventStatus, subscription)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/eventdeliveries?endpointId=%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpoint1.UID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.EventDelivery
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(respEvents))

	v := []*datastore.EventDelivery{d2, d1}
	for i, delivery := range v {
		require.Equal(s.T(), respEvents[i].UID, delivery.UID)
	}
}

func TestEventIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(EventIntegrationTestSuite))
}

type OrganisationIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *OrganisationIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *OrganisationIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-all-realms.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *OrganisationIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *OrganisationIntegrationTestSuite) Test_CreateOrganisation() {
	expectedStatusCode := http.StatusCreated

	body := strings.NewReader(`{"name":"new_org"}`)
	// Arrange.
	url := "/ui/organisations"
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var organisation datastore.Organisation
	parseResponse(s.T(), w.Result(), &organisation)

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	org, err := orgRepo.FetchOrganisationByID(context.Background(), organisation.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "new_org", org.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_CreateOrganisation_EmptyOrganisationName() {
	expectedStatusCode := http.StatusBadRequest

	body := strings.NewReader(`{"name":""}`)
	// Arrange.
	url := "/ui/organisations"
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationIntegrationTestSuite) Test_UpdateOrganisation_CustomDomain() {
	expectedStatusCode := http.StatusAccepted

	uid := ulid.Make().String()
	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.DB, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, org, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
	require.NoError(s.T(), err)

	body := strings.NewReader(`{"custom_domain":"https://abc.com"}`)
	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodPut, url, "", body)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var organisation datastore.Organisation
	parseResponse(s.T(), w.Result(), &organisation)

	require.NoError(s.T(), err)
	require.Equal(s.T(), "abc.com", organisation.CustomDomain.ValueOrZero())
}

func (s *OrganisationIntegrationTestSuite) Test_UpdateOrganisation() {
	expectedStatusCode := http.StatusAccepted

	uid := ulid.Make().String()
	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.DB, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, org, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser, Project: s.DefaultProject.UID})
	require.NoError(s.T(), err)

	body := strings.NewReader(`{"name":"update_org"}`)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodPut, url, "", body)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	organisation, err := orgRepo.FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "update_org", organisation.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisation() {
	expectedStatusCode := http.StatusOK

	uid := ulid.Make().String()
	seedOrg, err := testdb.SeedOrganisation(s.ConvoyApp.A.DB, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, seedOrg, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var organisation datastore.Organisation
	parseResponse(s.T(), w.Result(), &organisation)

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	org, err := orgRepo.FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)
	require.Equal(s.T(), seedOrg.Name, org.Name)
	require.Equal(s.T(), seedOrg.UID, organisation.UID)
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisations() {
	expectedStatusCode := http.StatusOK

	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.DB, ulid.Make().String(), s.DefaultUser.UID, "test-org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, org, s.DefaultUser, &auth.Role{Type: auth.RoleAdmin})
	require.NoError(s.T(), err)

	// Arrange.
	url := "/ui/organisations?page=1&perPage=2"
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var organisations []datastore.Organisation
	pagedResp := pagedResponse{Content: &organisations}
	parseResponse(s.T(), w.Result(), &pagedResp)

	require.Equal(s.T(), 2, len(organisations))

	uids := []string{s.DefaultOrg.UID, org.UID}
	for _, org := range organisations {
		require.Contains(s.T(), uids, org.UID)
	}
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisations_WithPersonalAPIKey() {
	//	expectedStatusCode := http.StatusOK
	//
	//	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.DB, ulid.Make().String(), s.DefaultUser.UID, "test-org")
	//	require.NoError(s.T(), err)
	//
	//	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, org, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
	//	require.NoError(s.T(), err)
	//
	//	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	//	require.NoError(s.T(), err)
	//
	//	// Arrange.
	//	url := "/ui/organisations/organisations?page=1&perPage=2"
	//	req := createRequest(http.MethodGet, url, key, nil)
	//
	//	w := httptest.NewRecorder()
	//
	//	// Act.
	//	s.Router.ServeHTTP(w, req)
	//
	//	// Assert.
	//	require.Equal(s.T(), expectedStatusCode, w.Code)
	//
	//	// Deep Assert.
	//	var organisations []datastore.Organisation
	//	pagedResp := pagedResponse{Content: &organisations}
	//	parseResponse(s.T(), w.Result(), &pagedResp)
	//
	//	require.Equal(s.T(), 2, len(organisations))
	//
	//	uids := []string{s.DefaultOrg.UID, org.UID}
	//	for _, org := range organisations {
	//		require.Contains(s.T(), uids, org.UID)
	//	}
}

func (s *OrganisationIntegrationTestSuite) Test_DeleteOrganisation() {
	expectedStatusCode := http.StatusOK

	uid := ulid.Make().String()
	seedOrg, err := testdb.SeedOrganisation(s.ConvoyApp.A.DB, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, seedOrg, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodDelete, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	_, err = orgRepo.FetchOrganisationByID(context.Background(), uid)
	require.Equal(s.T(), datastore.ErrOrgNotFound, err)
}

func TestOrganisationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationIntegrationTestSuite))
}

type OrganisationInviteIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *OrganisationInviteIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *OrganisationInviteIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *OrganisationInviteIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation() {
	expectedStatusCode := http.StatusCreated

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invites", s.DefaultOrg.UID)

	// TODO(daniel): when the generic mailer is integrated we have to mock it
	body := serialize(`{"invitee_email":"test@invite.com","role":{"type":"api", "project":"%s"}}`, s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation_InvalidRole() {
	expectedStatusCode := http.StatusBadRequest

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invites", s.DefaultOrg.UID)

	body := strings.NewReader(`{"invitee_email":"test@invite.com",role":{"type":"api"}}`)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation_InvalidEmail() {
	expectedStatusCode := http.StatusBadRequest

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invites", s.DefaultOrg.UID)

	body := strings.NewReader(`{"invitee_email":"test_invite.com",role":{"type":"api","project":"123"}}`)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation_EmptyEmail() {
	expectedStatusCode := http.StatusBadRequest

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invites", s.DefaultOrg.UID)

	body := strings.NewReader(`{"invitee_email":"",role":{"type":"api","project":"123"}}`)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_GetPendingOrganisationInvites() {
	expectedStatusCode := http.StatusOK

	_, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "invite1@test.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "invite2@test.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invites/pending", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var invites []datastore.OrganisationInvite
	pagedResp := pagedResponse{Content: &invites}
	parseResponse(s.T(), w.Result(), &pagedResp)

	require.Equal(s.T(), 2, len(invites))
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_AcceptForExistingUser() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "invite@test.com", "password")
	require.NoError(s.T(), err)

	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, user.Email, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)
	req := createRequest(http.MethodPost, url, "", nil)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_InviteExpired() {
	expectedStatusCode := http.StatusBadRequest

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "invite@test.com", "password")
	require.NoError(s.T(), err)

	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, user.Email, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(-time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)
	req := createRequest(http.MethodPost, url, "", nil)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_AcceptForNewUser() {
	expectedStatusCode := http.StatusOK

	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "test@invite.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)

	body := strings.NewReader(`{"first_name":"test","last_name":"test","email":"test@invite.com","password":"password"}`)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_EmptyFirstName() {
	expectedStatusCode := http.StatusBadRequest

	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "test@invite.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)

	body := strings.NewReader(`{"first_name":"","last_name":"test","email":"test@invite.com","password":"password"}`)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_Decline() {
	expectedStatusCode := http.StatusOK

	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "test@invite.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=false", iv.Token)
	req := createRequest(http.MethodPost, url, "", nil)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_FindUserByInviteToken_ExistingUser() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "invite@test.com", "password")
	require.NoError(s.T(), err)

	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, user.Email, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/users/token?token=%s", iv.Token)
	req := createRequest(http.MethodGet, url, "", nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response models.UserInviteTokenResponse
	parseResponse(s.T(), w.Result(), &response)

	require.Equal(s.T(), user.UID, response.User.UID)
	require.Equal(s.T(), user.FirstName, response.User.FirstName)
	require.Equal(s.T(), user.LastName, response.User.LastName)
	require.Equal(s.T(), user.Email, response.User.Email)
	require.Equal(s.T(), iv.UID, response.Token.UID)
	require.Equal(s.T(), iv.InviteeEmail, response.Token.InviteeEmail)
	require.Equal(s.T(), iv.Token, response.Token.Token)
	require.Equal(s.T(), response.Token.OrganisationName, s.DefaultOrg.Name)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_FindUserByInviteToken_NewUser() {
	expectedStatusCode := http.StatusOK

	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "invite@test.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/users/token?token=%s", iv.Token)
	req := createRequest(http.MethodGet, url, "", nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response models.UserInviteTokenResponse
	parseResponse(s.T(), w.Result(), &response)

	require.Equal(s.T(), iv.UID, response.Token.UID)
	require.Equal(s.T(), iv.InviteeEmail, response.Token.InviteeEmail)
	require.Equal(s.T(), iv.Token, response.Token.Token)
	require.Nil(s.T(), response.User)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ResendInvite() {
	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "invite1@test.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/invites/%s/resend", s.DefaultOrg.UID, iv.UID)

	req := createRequest(http.MethodPost, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_CancelInvite() {
	iv, err := testdb.SeedOrganisationInvite(s.ConvoyApp.A.DB, s.DefaultOrg, "invite1@test.com", &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	}, time.Now().Add(time.Hour), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/invites/%s/cancel", s.DefaultOrg.UID, iv.UID)

	req := createRequest(http.MethodPost, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var response datastore.OrganisationInvite
	parseResponse(s.T(), w.Result(), &response)
	require.Equal(s.T(), datastore.InviteStatusCancelled, response.Status)
}

func TestOrganisationInviteIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationInviteIntegrationTestSuite))
}

type OrganisationMemberIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *OrganisationMemberIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *OrganisationMemberIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *OrganisationMemberIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *OrganisationMemberIntegrationTestSuite) Test_GetOrganisationMembers() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	})
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var members []datastore.OrganisationMember
	pagedResp := pagedResponse{Content: &members}
	parseResponse(s.T(), w.Result(), &pagedResp)
	require.Equal(s.T(), 2, len(members))

	metadata := []datastore.UserMetadata{
		{
			FirstName: s.DefaultUser.FirstName,
			LastName:  s.DefaultUser.LastName,
			Email:     s.DefaultUser.Email,
		},
		{
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Email:     user.Email,
		},
	}

	for _, member := range members {
		require.Contains(s.T(), metadata, member.UserMetadata)
	}
}

func (s *OrganisationMemberIntegrationTestSuite) Test_GetOrganisationMember() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	member, err := testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAdmin})

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members/%s", s.DefaultOrg.UID, member.UID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var m datastore.OrganisationMember
	parseResponse(s.T(), w.Result(), &m)

	require.Equal(s.T(), member.UID, m.UID)
	require.Equal(s.T(), member.OrganisationID, m.OrganisationID)
	require.Equal(s.T(), member.UserID, m.UserID)

	require.Equal(s.T(), datastore.UserMetadata{
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Email:     user.Email,
	}, m.UserMetadata)
}

func (s *OrganisationMemberIntegrationTestSuite) Test_UpdateOrganisationMember() {
	expectedStatusCode := http.StatusAccepted

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	member, err := testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	})
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members/%s", s.DefaultOrg.UID, member.UID)

	body := serialize(`{"role":{ "type":"api", "project":"%s"}}`, s.DefaultProject.UID)
	req := createRequest(http.MethodPut, url, "", body)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var m datastore.OrganisationMember
	parseResponse(s.T(), w.Result(), &m)

	require.Equal(s.T(), member.UID, m.UID)
	require.Equal(s.T(), auth.Role{Type: auth.RoleAPI, Project: s.DefaultProject.UID}, m.Role)
}

func (s *OrganisationMemberIntegrationTestSuite) Test_DeleteOrganisationMember() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	member, err := testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  s.DefaultProject.UID,
		Endpoint: "",
	})
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members/%s", s.DefaultOrg.UID, member.UID)
	req := createRequest(http.MethodDelete, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	orgMemberRepo := postgres.NewOrgMemberRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	_, err = orgMemberRepo.FetchOrganisationMemberByID(context.Background(), member.UID, s.DefaultOrg.UID)
	require.Equal(s.T(), datastore.ErrOrgMemberNotFound, err)
}

func (s *OrganisationMemberIntegrationTestSuite) Test_CannotDeleteOrganisationOwner() {
	expectedStatusCode := http.StatusForbidden

	orgMemberRepo := postgres.NewOrgMemberRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	member, err := orgMemberRepo.FetchOrganisationMemberByUserID(context.Background(), s.DefaultUser.UID, s.DefaultOrg.UID)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members/%s", s.DefaultOrg.UID, member.UID)
	req := createRequest(http.MethodDelete, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func TestOrganisationMemberIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationMemberIntegrationTestSuite))
}

type PortalLinkIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *PortalLinkIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *PortalLinkIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalLinkIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PortalLinkIntegrationTestSuite) Test_CreatePortalLink() {
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/portal-links",
		s.DefaultProject.OrganisationID,
		s.DefaultProject.UID)
	plainBody := fmt.Sprintf(`{
		"name": "test_portal_link",
		"endpoints": ["%s", "%s"]
	}`, endpoint1.UID, endpoint2.UID)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_PortalLinkNotFound() {
	portalLinkID := "123"

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/portal-links/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLinkID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_ValidPortalLink() {
	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/portal-links/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLink.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
	require.Equal(s.T(), 1, resp.EndpointCount)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)

		_, err = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
		require.NoError(s.T(), err)
	}

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/portal-links", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalLinks, len(resp.Content.([]interface{})))
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks_FilterByEndpointID() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)

		_, err = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
		require.NoError(s.T(), err)
	}

	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/portal-links?endpointId=%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, endpoint.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 1, len(resp.Content.([]interface{})))
}

func (s *PortalLinkIntegrationTestSuite) Test_UpdatePortalLinks() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID})

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/portal-links/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLink.UID)
	bodyStr := fmt.Sprintf(`{
		    "name": "test_portal_link",
			"endpoints": ["%s"]
		}`, endpoint2.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.ProjectID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal?token=%s", pl.Token))
	require.Equal(s.T(), resp.Name, pl.Name)
	require.Equal(s.T(), resp.Endpoints, []string(pl.Endpoints))
}

func (s *PortalLinkIntegrationTestSuite) Test_RevokePortalLink() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint1.UID})

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/portal-links/%s/revoke", s.DefaultProject.OrganisationID, s.DefaultProject.UID, portalLink.UID)
	req := createRequest(http.MethodPut, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	plRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	_, err = plRepo.FindPortalLinkByID(context.Background(), s.DefaultProject.UID, portalLink.UID)
	require.ErrorIs(s.T(), err, datastore.ErrPortalLinkNotFound)
}

func TestPortalLinkIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalLinkIntegrationTestSuite))
}

type ProjectIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *ProjectIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *ProjectIntegrationTestSuite) SetupTest() {
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

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-all-realms.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *ProjectIntegrationTestSuite) TestGetProject() {
	projectID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "test-app", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, project.UID, ulid.Make().String(), "*", "", []byte("{}"))

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, project.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject datastore.Project
	parseResponse(s.T(), w.Result(), &respProject)
	require.Equal(s.T(), project.UID, respProject.UID)
}

func (s *ProjectIntegrationTestSuite) TestGetProject_ProjectNotFound() {
	expectedStatusCode := http.StatusBadRequest

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, ulid.Make().String())
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestDeleteProject() {
	projectID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "x-proj", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, project.UID)
	req := createRequest(http.MethodDelete, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
	projectRepo := postgres.NewProjectRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	_, err = projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.Equal(s.T(), datastore.ErrProjectNotFound, err)
}

func (s *ProjectIntegrationTestSuite) TestDeleteProject_ProjectNotFound() {
	expectedStatusCode := http.StatusBadRequest

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, ulid.Make().String())
	req := createRequest(http.MethodDelete, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestCreateProject() {
	expectedStatusCode := http.StatusCreated

	bodyStr := `{
    "name": "test-project",
	"type": "outgoing",
    "logo_url": "",
    "config": {
        "strategy": {
            "type": "linear",
            "duration": 10,
            "retry_count": 2
        },
        "signature": {
            "header": "X-Convoy-Signature",
            "hash": "SHA512"
        },
        "disable_endpoint": false,
        "replay_attacks": false,
        "ratelimit": {
            "count": 8000,
            "duration": 60
        }
    }
}`

	body := serialize(bodyStr)
	url := fmt.Sprintf("/ui/organisations/%s/projects", s.DefaultOrg.UID)

	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()
	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject models.CreateProjectResponse
	parseResponse(s.T(), w.Result(), &respProject)
	require.NotEmpty(s.T(), respProject.Project.UID)
	require.Equal(s.T(), 8000, respProject.Project.Config.RateLimit.Count)
	require.Equal(s.T(), uint64(60), respProject.Project.Config.RateLimit.Duration)
	require.Equal(s.T(), "test-project", respProject.Project.Name)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)

	require.Equal(s.T(), auth.RoleAdmin, respProject.APIKey.Role.Type)
	require.Equal(s.T(), respProject.Project.UID, respProject.APIKey.Role.Project)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)
	require.NotEmpty(s.T(), respProject.APIKey.Key)
}

func (s *ProjectIntegrationTestSuite) TestUpdateProject() {
	projectID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "x-proj", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, project.UID)

	bodyStr := `{
    "name": "project_1",
	"type": "outgoing",
    "config": {
        "retention_policy":{"policy":"1h"},
        "strategy": {
            "type": "exponential",
            "duration": 10,
            "retry_count": 2
        },
         "ssl": {
            "enforce_secure_endpoints": true
        },
        "signature": {
            "header": "X-Convoy-Signature",
            "hash": "SHA512"
        },
        "disable_endpoint": false,
        "replay_attacks": false,
        "ratelimit": {
            "count": 8000,
            "duration": 60
        }
    }
}`
	req := createRequest(http.MethodPut, url, "", serialize(bodyStr))
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
	projectRepo := postgres.NewProjectRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	g, err := projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "project_1", g.Name)
}

func (s *ProjectIntegrationTestSuite) TestGetProjects() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	project1, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "123", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	project2, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "43", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	project3, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "66", s.DefaultOrg.UID, datastore.OutgoingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var projects []*datastore.Project
	parseResponse(s.T(), w.Result(), &projects)
	require.Equal(s.T(), 4, len(projects))

	v := []string{projects[0].UID, projects[1].UID, projects[2].UID, projects[3].UID}
	require.Contains(s.T(), v, project1.UID)
	require.Contains(s.T(), v, project2.UID)
	require.Contains(s.T(), v, project3.UID)
	require.Contains(s.T(), v, s.DefaultProject.UID)
}

func (s *ProjectIntegrationTestSuite) TestGetProjectStats() {
	expectedStatusCode := http.StatusOK

	for i := 0; i < 2; i++ {
		source, err := testdb.SeedSource(s.DB, s.DefaultProject, "", "", "", nil, "", "")
		require.NoError(s.T(), err)

		endpoint, err := testdb.SeedEndpoint(s.DB, s.DefaultProject, "", "", "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)

		_, err = testdb.SeedSubscription(s.DB, s.DefaultProject, "", datastore.IncomingProject, source, endpoint, &datastore.DefaultRetryConfig, &datastore.DefaultAlertConfig, nil)
		require.NoError(s.T(), err)
	}

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/stats", s.DefaultOrg.UID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var stats *datastore.ProjectStatistics
	parseResponse(s.T(), w.Result(), &stats)

	require.Equal(s.T(), true, stats.TotalEndpoints)     // int64(2)
	require.Equal(s.T(), true, stats.TotalSources)       // int64(2)
	require.Equal(s.T(), true, stats.TotalSubscriptions) // int64(2)
}

func (s *ProjectIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func TestProjectIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectIntegrationTestSuite))
}

type SourceIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *SourceIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *SourceIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *SourceIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *SourceIntegrationTestSuite) Test_GetSourceByID_SourceNotFound() {
	sourceID := "123"

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodGet, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SourceIntegrationTestSuite) Test_GetSourceBy_ValidSource() {
	sourceID := "123456789"

	// Just Before
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil, "", "")

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodGet, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbSource, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), source.UID, dbSource.UID)
	require.Equal(s.T(), source.Name, dbSource.Name)
	require.Equal(s.T(), source.MaskID, dbSource.MaskID)
}

func (s *SourceIntegrationTestSuite) Test_GetSource_ValidSources() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalSources := r.Intn(5)

	// Just Before
	for i := 0; i < totalSources; i++ {
		_, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil, "", "")
		require.NoError(s.T(), err)
	}

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalSources, len(resp.Content.([]interface{})))
}

func (s *SourceIntegrationTestSuite) Test_CreateSource() {
	bodyStr := `{
		"name": "convoy-prod",
		"type": "http",
		"is_disabled": false,
        "custom_response": {
            "body": "[accepted]",
            "content_type": "text/plain"
        },
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	require.NotEmpty(s.T(), source.UID)
	require.Equal(s.T(), "convoy-prod", source.Name)
	require.Equal(s.T(), datastore.SourceType("http"), source.Type)
	require.Equal(s.T(), datastore.VerifierType("hmac"), source.Verifier.Type)
	require.Equal(s.T(), "[accepted]", source.CustomResponse.Body)
	require.Equal(s.T(), "text/plain", source.CustomResponse.ContentType)
}

func (s *SourceIntegrationTestSuite) Test_CreateSource_NoName() {
	bodyStr := `{
		"type": "http",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SourceIntegrationTestSuite) Test_CreateSource_InvalidSourceType() {
	bodyStr := `{
		"name": "convoy-prod",
		"type": "some-random-source-type",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources", s.DefaultProject.OrganisationID, s.DefaultProject.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SourceIntegrationTestSuite) Test_UpdateSource() {
	name := "updated-convoy-prod"
	isDisabled := randBool()
	sourceID := ulid.Make().String()

	// Just Before
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil, "", "")

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, sourceID)
	bodyStr := fmt.Sprintf(`{
		"name": "%s",
		"type": "http",
		"is_disabled": %t,
        "custom_response": {
            "body": "[tee]",
            "content_type": "text/plain"
        },
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "hex",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`, name, !isDisabled)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbSource, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), source.UID, dbSource.UID)
	require.Equal(s.T(), name, dbSource.Name)
	require.Equal(s.T(), !isDisabled, dbSource.IsDisabled)
	require.Equal(s.T(), "[tee]", source.CustomResponse.Body)
	require.Equal(s.T(), "text/plain", source.CustomResponse.ContentType)
}

func (s *SourceIntegrationTestSuite) Test_DeleteSource() {
	sourceID := ulid.Make().String()

	// Just Before.
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil, "", "")

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/sources/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodDelete, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	_, err = sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.ErrorIs(s.T(), err, datastore.ErrSourceNotFound)
}

func TestSourceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SourceIntegrationTestSuite))
}

type SubscriptionIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *SubscriptionIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *SubscriptionIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-all-realms.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *SubscriptionIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription() {
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	body := serialize(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"project_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"duration": "10s",
			"interval_seconds": 10
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		},
		"rate_limit_config": {
			"count": 100,
			"duration": 5
		},
		"disable_endpoint": true
	}`, endpoint.UID, s.DefaultProject.UID, endpoint.UID)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions",
		s.DefaultProject.OrganisationID,
		s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_IncomingProject() {
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	source, _ := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil, "", "")
	body := serialize(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
        "source_id":"%s",
		"project_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"duration": "10s"
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		},
		"rate_limit_config": {
			"count": 100,
			"duration": 5
		}
	}`, endpoint.UID, source.UID, s.DefaultProject.UID, endpoint.UID)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions",
		s.DefaultProject.OrganisationID,
		s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscription.UID)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), subscription.UID)
	require.Equal(s.T(), dbSub.Name, subscription.Name)
	require.Equal(s.T(), len(dbSub.FilterConfig.EventTypes), len(subscription.FilterConfig.EventTypes))
	require.Equal(s.T(), dbSub.RateLimitConfig.Count, subscription.RateLimitConfig.Count)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_EndpointNotFound() {
	body := serialize(`{
		"name": "sub-1",
		"type": "incoming",
		"app_id": "%s",
		"project_id": "%s",
		"endpoint_id": "%s",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"interval_seconds": 10
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		}
	}`, ulid.Make().String(), s.DefaultProject.UID, ulid.Make().String())

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions",
		s.DefaultProject.OrganisationID,
		s.DefaultProject.UID)
	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_CreateSubscription_InvalidBody() {
	bodyStr := `{
		"name": "sub-1",
		"type": "incoming",
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 2,
			"interval_seconds": 10
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		}
	}`

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_SubscriptionNotFound() {
	subscriptionId := "123"

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, subscriptionId)
	req := createRequest(http.MethodGet, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_OutgoingProject_ValidSubscription() {
	subscriptionId := ulid.Make().String()

	project := s.DefaultProject

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil, "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, project, subscriptionId, project.Type, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, subscriptionId)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetOneSubscription_IncomingProject_ValidSubscription() {
	subscriptionId := ulid.Make().String()

	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test-project", s.DefaultOrg.UID, datastore.IncomingProject, &datastore.DefaultProjectConfig)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: project.UID,
	}

	_, apiKey, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, project, ulid.Make().String(), "", "", nil, "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, project, subscriptionId, "incoming", source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions/%s", project.OrganisationID, project.UID, subscriptionId)
	req := createRequest(http.MethodGet, url, apiKey, nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), project.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), subscription.UID, dbSub.UID)
	require.Equal(s.T(), subscription.Source.UID, dbSub.SourceID)
	require.Equal(s.T(), subscription.Endpoint.UID, dbSub.EndpointID)
}

func (s *SubscriptionIntegrationTestSuite) Test_GetSubscriptions_ValidSubscriptions() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalSubs := r.Intn(10)

	for i := 0; i < totalSubs; i++ {
		// Just Before
		endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
		require.NoError(s.T(), err)
		source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil, "", "")
		require.NoError(s.T(), err)

		_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
		require.NoError(s.T(), err)
	}

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, "", nil)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), totalSubs, len(resp.Content.([]interface{})))
}

func (s *SubscriptionIntegrationTestSuite) Test_DeleteSubscription() {
	subscriptionId := ulid.Make().String()

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil, "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, subscriptionId, datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request.
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, subscriptionId)
	req := createRequest(http.MethodDelete, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	_, err = subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.ErrorIs(s.T(), err, datastore.ErrSubscriptionNotFound)
}

func (s *SubscriptionIntegrationTestSuite) Test_UpdateSubscription() {
	subscriptionId := ulid.Make().String()

	// Just Before
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	source, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, ulid.Make().String(), "", "", nil, "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedSubscription(s.ConvoyApp.A.DB, s.DefaultProject, subscriptionId, datastore.OutgoingProject, source, endpoint, &datastore.RetryConfiguration{}, &datastore.AlertConfiguration{}, nil)
	require.NoError(s.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/subscriptions/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, subscriptionId)
	bodyStr := `{
		"alert_config": {
			"threshold": "1h",
			"count": 10
		},
		"retry_config": {
			"type": "linear",
			"retry_count": 3,
			"duration": "2s"
		},
		"filter_config": {
			"event_types": [
				"user.created",
				"user.updated"
			]
		},
		"disable_endpoint": false
	}`

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, "", body)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var subscription *datastore.Subscription
	parseResponse(s.T(), w.Result(), &subscription)

	subRepo := postgres.NewSubscriptionRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	dbSub, err := subRepo.FindSubscriptionByID(context.Background(), s.DefaultProject.UID, subscriptionId)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, len(dbSub.FilterConfig.EventTypes))
	require.Equal(s.T(), "1h", dbSub.AlertConfig.Threshold)
	require.Equal(s.T(), subscription.RetryConfig.Duration, dbSub.RetryConfig.Duration)
}

func TestSubscriptionIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionIntegrationTestSuite))
}

type UserIntegrationTestSuite struct {
	suite.Suite
	DB        database.Database
	Router    http.Handler
	ConvoyApp *ApplicationHandler
	jwt       *jwt.Jwt
}

func (u *UserIntegrationTestSuite) SetupSuite() {
	u.DB = getDB()
	u.ConvoyApp = buildServer()
	u.Router = u.ConvoyApp.BuildControlPlaneRoutes()
}

func (u *UserIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(u.T(), u.DB)

	err := config.LoadConfig("./testdata/Auth_Config/jwt-convoy.json")
	require.NoError(u.T(), err)
	require.NoError(u.T(), err)

	configuration, err := config.Get()

	u.jwt = jwt.NewJwt(&configuration.Auth.Jwt, u.ConvoyApp.A.Cache)

	apiRepo := postgres.NewAPIKeyRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	initRealmChain(u.T(), apiRepo, userRepo, portalLinkRepo, u.ConvoyApp.A.Cache)
}

func (u *UserIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(u.T(), u.DB)
	metrics.Reset()
}

func (u *UserIntegrationTestSuite) Test_RegisterUser() {
	_, err := testdb.SeedConfiguration(u.ConvoyApp.A.DB)
	require.NoError(u.T(), err)

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"email": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.FirstName, r.LastName, r.Email, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusCreated, w.Code)

	var response models.LoginUserResponse
	parseResponse(u.T(), w.Result(), &response)

	require.NotEmpty(u.T(), response.UID)
	require.NotEmpty(u.T(), response.Token.AccessToken)
	require.NotEmpty(u.T(), response.Token.RefreshToken)

	require.Equal(u.T(), r.FirstName, response.FirstName)
	require.Equal(u.T(), r.LastName, response.LastName)
	require.Equal(u.T(), r.Email, response.Email)

	dbUser, err := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache).FindUserByID(context.Background(), response.UID)
	require.NoError(u.T(), err)
	require.False(u.T(), dbUser.EmailVerified)
	require.NotEmpty(u.T(), dbUser.EmailVerificationToken)
	require.NotEmpty(u.T(), dbUser.EmailVerificationExpiresAt)
}

func (u *UserIntegrationTestSuite) Test_RegisterUser_RegistrationNotAllowed() {
	configuration, err := testdb.SeedConfiguration(u.ConvoyApp.A.DB)
	require.NoError(u.T(), err)

	// disable registration
	configuration.IsSignupEnabled = false
	configRepo := postgres.NewConfigRepo(u.ConvoyApp.A.DB)
	require.NoError(u.T(), configRepo.UpdateConfiguration(context.Background(), configuration))

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"email": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.FirstName, r.LastName, r.Email, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusForbidden, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RegisterUser_NoFirstName() {
	_, err := testdb.SeedConfiguration(u.ConvoyApp.A.DB)
	require.NoError(u.T(), err)

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"last_name": "%s",
		"email": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.LastName, r.Email, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RegisterUser_NoEmail() {
	_, err := testdb.SeedConfiguration(u.ConvoyApp.A.DB)
	require.NoError(u.T(), err)

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.FirstName, r.LastName, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_GetUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/profile", user.UID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)

	require.Equal(u.T(), user.UID, response.UID)
	require.Equal(u.T(), user.FirstName, response.FirstName)
	require.Equal(u.T(), user.LastName, response.LastName)
	require.Equal(u.T(), user.Email, response.Email)
}

func (u *UserIntegrationTestSuite) Test_UpdateUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	user.EmailVerified = true
	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)

	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	firstName := fmt.Sprintf("test%s", ulid.Make().String())
	lastName := fmt.Sprintf("test%s", ulid.Make().String())
	email := fmt.Sprintf("%s@test.com", ulid.Make().String())

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/profile", user.UID)
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"email": "%s"
	}`, firstName, lastName, email)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)

	dbUser, err := userRepo.FindUserByID(context.Background(), user.UID)

	require.Equal(u.T(), dbUser.UID, response.UID)
	require.Equal(u.T(), firstName, dbUser.FirstName)
	require.Equal(u.T(), lastName, dbUser.LastName)
	require.Equal(u.T(), email, dbUser.Email)
}

func (u *UserIntegrationTestSuite) Test_UpdatePassword() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	user.EmailVerified = true

	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	newPassword := "123456789"

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/password", user.UID)
	bodyStr := fmt.Sprintf(`{
		"current_password": "%s",
		"password": "%s",
		"password_confirmation": "%s"
	}`, password, newPassword, newPassword)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)

	dbUser, err := userRepo.FindUserByID(context.Background(), user.UID)

	p := datastore.Password{Plaintext: newPassword, Hash: []byte(dbUser.Password)}
	isMatch, err := p.Matches()

	require.NoError(u.T(), err)

	require.Equal(u.T(), dbUser.UID, response.UID)
	require.True(u.T(), isMatch)
}

func (u *UserIntegrationTestSuite) Test_UpdatePassword_Invalid_Current_Password() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	user.EmailVerified = true

	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/password", user.UID)
	bodyStr := fmt.Sprintf(`{
		"current_password": "new-password",
		"password": "%s",
		"password_confirmation": "%s"
	}`, password, password)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_UpdatePassword_Invalid_Password_Confirmation() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	user.EmailVerified = true

	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/password", user.UID)
	bodyStr := fmt.Sprintf(`{
		"current_password": %s,
		"password": "%s",
		"password_confirmation": "new-password"
	}`, password, password)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_Forgot_Password_Valid_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	newPassword := "123456789"

	// Arrange Request
	url := "/ui/users/forgot-password"
	bodyStr := fmt.Sprintf(`{"email":"%s"}`, user.Email)

	req := httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	dbUser, err := userRepo.FindUserByEmail(context.Background(), user.Email)
	require.NoError(u.T(), err)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)
	// Reset password
	url = fmt.Sprintf("/ui/users/reset-password?token=%s", dbUser.ResetPasswordToken)
	bodyStr = fmt.Sprintf(`{
		"password": "%s",
		"password_confirmation": "%s"
	}`, newPassword, newPassword)

	req = httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w = httptest.NewRecorder()
	u.Router.ServeHTTP(w, req)
	require.Equal(u.T(), http.StatusOK, w.Code)
	parseResponse(u.T(), w.Result(), &response)

	dbUser, err = userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(u.T(), err)

	p := datastore.Password{Plaintext: newPassword, Hash: []byte(dbUser.Password)}
	isMatch, err := p.Matches()

	require.NoError(u.T(), err)
	require.Equal(u.T(), dbUser.UID, response.UID)
	require.True(u.T(), isMatch)
}

func (u *UserIntegrationTestSuite) Test_Forgot_Password_Invalid_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	newPassword := "123456789"

	// Arrange Request
	url := "/ui/users/forgot-password"
	bodyStr := fmt.Sprintf(`{"email":"%s"}`, user.Email)

	req := httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	// Reset password
	url = fmt.Sprintf("/ui/users/reset-password?token=%s", "fake-token")
	bodyStr = fmt.Sprintf(`{
		"password": "%s",
		"password_confirmation": "%s"
	}`, newPassword, newPassword)

	req = httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w = httptest.NewRecorder()
	u.Router.ServeHTTP(w, req)
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_VerifyEmail() {
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", testdb.DefaultUserPassword)

	user.EmailVerificationToken = ulid.Make().String()
	user.EmailVerificationExpiresAt = time.Now().Add(time.Hour)

	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB, u.ConvoyApp.A.Cache)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/verify_email?token=%s", user.EmailVerificationToken)

	req := createRequest(http.MethodPost, url, "", nil)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	dbUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(u.T(), err)
	require.True(u.T(), dbUser.EmailVerified)
}

func TestUserIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(UserIntegrationTestSuite))
}

type MetaEventIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *MetaEventIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *MetaEventIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)
	s.DB = getDB()

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, org.UID)
	require.NoError(s.T(), err)

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB, s.ConvoyApp.A.Cache)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *MetaEventIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *MetaEventIntegrationTestSuite) Test_GetMetaEventsPaged() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	m1, err := testdb.SeedMetaEvent(s.ConvoyApp.A.DB, s.DefaultProject)
	require.NoError(s.T(), err)

	m2, err := testdb.SeedMetaEvent(s.ConvoyApp.A.DB, s.DefaultProject)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/meta-events", s.DefaultProject.OrganisationID, s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respEvents []datastore.MetaEvent
	resp := pagedResponse{Content: &respEvents}
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), 2, len(respEvents))

	v := []string{m1.UID, m2.UID}
	for i := range respEvents {
		require.Contains(s.T(), v, respEvents[i].UID)
	}
}

func (s *MetaEventIntegrationTestSuite) Test_GetMetaEvent_Valid_MetaEvent() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	metaEvent, err := testdb.SeedMetaEvent(s.ConvoyApp.A.DB, s.DefaultProject)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s/meta-events/%s", s.DefaultProject.OrganisationID, s.DefaultProject.UID, metaEvent.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var respMetaEvent datastore.MetaEvent
	parseResponse(s.T(), w.Result(), &respMetaEvent)
	require.Equal(s.T(), metaEvent.UID, respMetaEvent.UID)
}

func TestMetaEventIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(MetaEventIntegrationTestSuite))
}
