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

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ApplicationIntegrationTestSuite struct {
	suite.Suite
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *applicationHandler
	DefaultGroup *datastore.Group
}

func (s *ApplicationIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *ApplicationIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB)

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo())
}

func (s *ApplicationIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApp_AppNotFound() {
	appID := "123"
	expectedStatusCode := http.StatusNotFound

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", appID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.SetBasicAuth("test", "test")
	req.Header.Add("Content-Type", "application/json")
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
	_, _ = testdb.SeedApplication(s.DB, s.DefaultGroup, appID, true)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/applications/%s", appID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.SetBasicAuth("test", "test")
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := s.DB.AppRepo()
	dbApp, err := appRepo.FindApplicationByID(context.Background(), appID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), app.UID, dbApp.UID)
	require.Equal(s.T(), app.Title, dbApp.Title)
}

func (s *ApplicationIntegrationTestSuite) Test_GetApps_ValidApplications() {
	rand.Seed(time.Now().UnixNano())
	totalApps := rand.Intn(5)
	expectedStatusCode := http.StatusOK

	// Just Before.
	_ = testdb.SeedMultipleApplications(s.DB, s.DefaultGroup, totalApps)

	// Arrange.
	url := "/api/v1/applications"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.SetBasicAuth("test", "test")
	req.Header.Add("Content-Type", "application/json")
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
	url := "/api/v1/applications"
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, appTitle)
	body := strings.NewReader(plainBody)
	req := httptest.NewRequest(http.MethodPost, url, body)
	req.SetBasicAuth("test", "test")
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var app datastore.Application
	parseResponse(s.T(), w.Result(), &app)

	appRepo := s.DB.AppRepo()
	dbApp, err := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), dbApp.Title, appTitle)
}

func (s *ApplicationIntegrationTestSuite) Test_CreateApp_NoName() {
	appTitle := ""
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := "/api/v1/applications"
	plainBody := fmt.Sprintf(`{
		"name": "%s"
	}`, appTitle)
	body := strings.NewReader(plainBody)
	req := httptest.NewRequest(http.MethodPost, url, body)
	req.SetBasicAuth("test", "test")
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ApplicationIntegrationTestSuite) Test_UpdateApp() {

}

func TestApplicationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ApplicationIntegrationTestSuite))
}
