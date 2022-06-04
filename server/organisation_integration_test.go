//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
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
	DB              datastore.DatabaseClient
	Router          http.Handler
	ConvoyApp       *applicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultGroup    *datastore.Group
	DefaultUser     *datastore.User
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

	user, err := testdb.SeedDefaultUser(s.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)
}

func (s *OrganisationIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *OrganisationIntegrationTestSuite) Test_CreateOrganisation() {
	expectedStatusCode := http.StatusCreated

	body := strings.NewReader(`{"name":"new_org"}`)
	// Arrange.
	url := "/ui/organisations"
	req := createRequest(http.MethodPost, url, body)
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

	org, err := s.DB.OrganisationRepo().FetchOrganisationByID(context.Background(), organisation.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "new_org", org.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_CreateOrganisation_EmptyOrganisationName() {
	expectedStatusCode := http.StatusBadRequest

	body := strings.NewReader(`{"name":""}`)
	// Arrange.
	url := "/ui/organisations"
	req := createRequest(http.MethodPost, url, body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationIntegrationTestSuite) Test_UpdateOrganisation_EmptyOrganisationName() {
	expectedStatusCode := http.StatusBadRequest

	uid := uuid.NewString()
	_, err := testdb.SeedOrganisation(s.DB, uid, "", "new_org")
	require.NoError(s.T(), err)

	body := strings.NewReader(`{"name":""}`)
	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodPut, url, body)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationIntegrationTestSuite) Test_UpdateOrganisation() {
	expectedStatusCode := http.StatusAccepted

	uid := uuid.NewString()
	_, err := testdb.SeedOrganisation(s.DB, uid, "", "new_org")
	require.NoError(s.T(), err)

	body := strings.NewReader(`{"name":"update_org"}`)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodPut, url, body)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

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
	expectedStatusCode := http.StatusOK

	uid := uuid.NewString()
	_, err := testdb.SeedOrganisation(s.DB, uid, "", "new_org")
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodGet, url, nil)
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

	org, err := s.DB.OrganisationRepo().FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "new_org", org.Name)
	require.Equal(s.T(), "new_org", organisation.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisations() {
	expectedStatusCode := http.StatusOK

	_, err := testdb.SeedMultipleOrganisations(s.DB, "", 5)
	require.NoError(s.T(), err)

	// Arrange.
	url := "/ui/organisations?page=2&perPage=2"
	req := createRequest(http.MethodGet, url, nil)
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
	require.Equal(s.T(), int64(5), pagedResp.Pagination.Total)
}

func (s *OrganisationIntegrationTestSuite) Test_DeleteOrganisation() {
	expectedStatusCode := http.StatusOK

	uid := uuid.NewString()
	_, err := testdb.SeedOrganisation(s.DB, uid, "", "new_org")
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s", uid)
	req := createRequest(http.MethodDelete, url, nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

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
