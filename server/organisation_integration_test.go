package server

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type OrganisationIntegrationTestSuite struct {
	suite.Suite
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *applicationHandler
	DefaultGroup *datastore.Group
}

func (s *OrganisationIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *OrganisationIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB)

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo())
}

func (s *OrganisationIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *OrganisationIntegrationTestSuite) Test_CreateOrganisation() {
	expectedStatusCode := http.StatusCreated

	body := strings.NewReader(`{"name":"new_org"}`)
	// Arrange.
	url := "/api/v1/organisations"
	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var organisation datastore.Organisation
	parseResponse(s.T(), w.Result(), &organisation)

	org, err := s.DB.OrganisationRepo().FetchOrganisationByID(context.Background(), organisation.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "new_org", org.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_UpdateOrganisation() {
	expectedStatusCode := http.StatusAccepted

	uid := uuid.NewString()
	_, err := testdb.SeedOrganisation(s.DB, uid, "", "new_org")
	require.NoError(s.T(), err)

	body := strings.NewReader(`{"name":"update_org"}`)

	// Arrange.
	url := fmt.Sprintf("/api/v1/organisations/%s", uid)
	req := createRequest(http.MethodPut, url, body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	org, err := s.DB.OrganisationRepo().FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "update_org", org.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisation() {
	expectedStatusCode := http.StatusAccepted

	uid := uuid.NewString()
	_, err := testdb.SeedOrganisation(s.DB, uid, "", "new_org")
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/api/v1/organisations/%s", uid)
	req := createRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var organisation datastore.Organisation
	parseResponse(s.T(), w.Result(), &organisation)

	org, err := s.DB.OrganisationRepo().FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "new_org", org.Name)
	require.Equal(s.T(), "new_org", organisation.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisations() {
	expectedStatusCode := http.StatusAccepted

	_, err := testdb.SeedOrganisation(s.DB, uuid.NewString(), "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisation(s.DB, uuid.NewString(), "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisation(s.DB, uuid.NewString(), "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisation(s.DB, uuid.NewString(), "", "")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisation(s.DB, uuid.NewString(), "", "")
	require.NoError(s.T(), err)

	// Arrange.
	url := "/api/v1/organisations?page=2&perPage=2"
	req := createRequest(http.MethodGet, url, nil)
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
	require.Equal(s.T(), 5, pagedResp.Pagination.Total)
}

func (s *OrganisationIntegrationTestSuite) Test_DeleteOrganisation() {
	expectedStatusCode := http.StatusAccepted

	uid := uuid.NewString()
	_, err := testdb.SeedOrganisation(s.DB, uid, "", "new_org")
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/api/v1/organisations/%s", uid)
	req := createRequest(http.MethodDelete, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	org, err := s.DB.OrganisationRepo().FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)

	require.NotEmpty(s.T(), org.DeletedAt)
	require.Equal(s.T(), datastore.DeletedDocumentStatus, org.DocumentStatus)
}

func TestOrganisationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationIntegrationTestSuite))
}
