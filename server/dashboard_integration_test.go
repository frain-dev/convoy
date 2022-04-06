//go:build integration
// +build integration

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	mongoStore "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestGetDashboardSummary(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	app := provideApplication(ctrl)

	db, closeFn := getDB(t)
	defer closeFn()

	app.groupRepo = mongoStore.NewGroupRepo(db)
	app.appRepo = mongoStore.NewApplicationRepo(db)
	app.eventRepo = mongoStore.NewEventRepository(db)
	app.groupRepo = mongoStore.NewGroupRepo(db)

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
	err := app.groupRepo.CreateGroup(ctx, group)
	require.NoError(t, err)

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

	err = app.appRepo.CreateApplication(ctx, application)
	require.NoError(t, err)

	events := []datastore.Event{
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			AppMetadata: &datastore.AppMetadata{
				UID:          application.UID,
				Title:        application.Title,
				GroupID:      group.UID,
				SupportEmail: application.SupportEmail,
			},
			CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.January, 1, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.January, 1, 1, 1, 1, 0, time.UTC)),
			DocumentStatus: datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			AppMetadata: &datastore.AppMetadata{
				UID:          application.UID,
				Title:        application.Title,
				GroupID:      group.UID,
				SupportEmail: application.SupportEmail,
			},
			CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.January, 10, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2021, time.January, 10, 1, 1, 1, 0, time.UTC)),
			DocumentStatus: datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			AppMetadata: &datastore.AppMetadata{
				UID:          application.UID,
				Title:        application.Title,
				GroupID:      group.UID,
				SupportEmail: application.SupportEmail,
			},
			CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus: datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			AppMetadata: &datastore.AppMetadata{
				UID:          application.UID,
				Title:        application.Title,
				GroupID:      group.UID,
				SupportEmail: application.SupportEmail,
			},
			CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus: datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			AppMetadata: &datastore.AppMetadata{
				UID:          application.UID,
				Title:        application.Title,
				GroupID:      group.UID,
				SupportEmail: application.SupportEmail,
			},
			CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus: datastore.ActiveDocumentStatus,
		},
		{
			UID:              uuid.New().String(),
			EventType:        "*",
			MatchedEndpoints: 1,
			ProviderID:       "provider_id",
			Data:             json.RawMessage(`{"data":"12345"}`),
			AppMetadata: &datastore.AppMetadata{
				UID:          application.UID,
				Title:        application.Title,
				GroupID:      group.UID,
				SupportEmail: application.SupportEmail,
			},
			CreatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			UpdatedAt:      primitive.NewDateTimeFromTime(time.Date(2022, time.March, 20, 1, 1, 1, 0, time.UTC)),
			DocumentStatus: datastore.ActiveDocumentStatus,
		},
	}

	for i := range events {
		err = app.eventRepo.CreateEvent(ctx, &events[i])
		require.NoError(t, err)
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
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig("./testdata/Auth_Config/none-convoy.json")
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, app.apiKeyRepo)

			req := httptest.NewRequest(tc.method, fmt.Sprintf("/ui/dashboard/summary?startDate=%s&endDate=%s&type=%s&groupId=%s", tc.urlQuery.startDate, tc.urlQuery.endDate, tc.urlQuery.Type, tc.urlQuery.groupID), nil)
			w := httptest.NewRecorder()

			router := buildRoutes(app)
			router.ServeHTTP(w, req)

			if w.Code != tc.statusCode {
				log.Error(tc.name, w.Body)
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, w.Code)
			}

			verifyMatch(t, *w)
		})
	}
}
