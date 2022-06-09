//go:build integration
// +build integration

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/suite"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/sebdah/goldie/v2"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DashboardIntegrationTestSuite struct {
	suite.Suite
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *applicationHandler
	DefaultGroup *datastore.Group
}

func (s *DashboardIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *DashboardIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB)

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)
}

func (s *DashboardIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *DashboardIntegrationTestSuite) TestGetDashboardSummary() {
	group := &datastore.Group{
		UID:               uuid.New().String(),
		Name:              "test-group",
		RateLimit:         3000,
		RateLimitDuration: "1m",
		CreatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:         primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus:    datastore.ActiveDocumentStatus,
	}

	ctx := context.Background()
	err := s.DB.GroupRepo().CreateGroup(ctx, group)
	require.NoError(s.T(), err)

	application := &datastore.Application{
		UID:            "abc",
		GroupID:        group.UID,
		Title:          "test-app",
		SupportEmail:   "test@suport.com",
		Endpoints:      []datastore.Endpoint{},
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	err = s.DB.AppRepo().CreateApplication(ctx, application, application.GroupID)
	require.NoError(s.T(), err)

	events := []datastore.Event{
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			GroupID:          group.UID,
			AppID:            application.UID,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Date(2021, time.January, 1, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Date(2021, time.January, 1, 1, 1, 1, 0, time.UTC)),
			DocumentStatus:   datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			GroupID:          group.UID,
			AppID:            application.UID,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Date(2021, time.January, 10, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Date(2021, time.January, 10, 1, 1, 1, 0, time.UTC)),
			DocumentStatus:   datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			GroupID:          group.UID,
			AppID:            application.UID,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus:   datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			GroupID:          group.UID,
			AppID:            application.UID,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus:   datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			GroupID:          group.UID,
			AppID:            application.UID,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus:   datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			GroupID:          group.UID,
			AppID:            application.UID,
			CreatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:        primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus:   datastore.ActiveDocumentStatus,
		},
	}

	for i := range events {
		err = s.DB.EventRepo().CreateEvent(ctx, &events[i])
		require.NoError(s.T(), err)
	}

	type urlQuery struct {
		groupID   string
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
				groupID:   group.UID,
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
				groupID:   group.UID,
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
				groupID:   group.UID,
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
				groupID:   group.UID,
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
				groupID: group.UID,
				endDate: "2022-12-27T00:00:00",
				Type:    "daily",
			},
		},
		{
			name:       "should_error_for_invalid_startDate",
			method:     http.MethodGet,
			statusCode: http.StatusBadRequest,
			urlQuery: urlQuery{
				groupID:   group.UID,
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
				groupID:   group.UID,
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
				groupID:   group.UID,
				startDate: "2021-01-01T00:00:00",
				endDate:   "2020-12-27T00:00:00",
				Type:      "daily",
			},
		},
	}
	for _, tc := range tests {
		s.T().Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)

			req := httptest.NewRequest(tc.method, fmt.Sprintf("/ui/dashboard/summary?startDate=%s&endDate=%s&type=%s&groupId=%s", tc.urlQuery.startDate, tc.urlQuery.endDate, tc.urlQuery.Type, tc.urlQuery.groupID), nil)
			req.SetBasicAuth("test", "test")
			w := httptest.NewRecorder()

			s.Router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				log.Error(tc.name, w.Body)
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
