//go:build integration
// +build integration

package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IngestIntegrationTestSuite struct {
	suite.Suite
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *Server
	DefaultGroup *datastore.Group
}

func (s *IngestIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.SetupRoutes()
}

func (i *IngestIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(i.DB)

	// Setup Default Group.
	i.DefaultGroup, _ = testdb.SeedDefaultGroup(i.DB, "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(i.T(), err)

	initRealmChain(i.T(), i.DB.APIRepo(), i.DB.UserRepo(), i.ConvoyApp.cache)
}

func (i *IngestIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(i.DB)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_BadMaskID() {
	maskID := "12345"
	expectedStatusCode := http.StatusBadRequest

	// Arrange Request.
	url := fmt.Sprintf("/ingest/%s", maskID)
	req := createRequest(http.MethodPost, url, "", nil)
	w := httptest.NewRecorder()

	// Act.
	i.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(i.T(), expectedStatusCode, w.Code)
}

func (i *IngestIntegrationTestSuite) Test_IngestEvent_BadRequest() {

}
