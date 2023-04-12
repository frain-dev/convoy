//go:build integration
// +build integration

package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

type DashboardIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *DashboardHandler
	AuthenticatorFn AuthenticatorFn
	DefaultUser     *datastore.User
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
}

func (s *DashboardIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
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
	err = config.LoadConfig("../testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
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
		Title:        "test-app",
		Secrets:      datastore.Secrets{},
		SupportEmail: "test@suport.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	endpointRepo := postgres.NewEndpointRepo(s.ConvoyApp.A.DB)
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

	eventDelivery := postgres.NewEventDeliveryRepo(s.ConvoyApp.A.DB)
	for i := range eventDeliveries {
		err = eventDelivery.CreateEventDelivery(ctx, &eventDeliveries[i])
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
			err := config.LoadConfig("../testdata/Auth_Config/full-convoy-with-jwt-realm.json")
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
			userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
			initRealmChain(t, apiRepo, userRepo, s.ConvoyApp.A.Cache)

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
		goldie.WithDiffEngine(goldie.ColoredDiff),
	)
	g.Assert(t, t.Name(), w.Body.Bytes())
}
