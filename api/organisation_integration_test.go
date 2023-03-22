//go:build integration
// +build integration

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

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
	s.Router = s.ConvoyApp.BuildRoutes()
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

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
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

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB)
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

	body := strings.NewReader(`{"custom_domain":"http://abc.com"}`)
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

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB)
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

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB)
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
	expectedStatusCode := http.StatusOK

	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.DB, ulid.Make().String(), s.DefaultUser.UID, "test-org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, org, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	// Arrange.
	url := "/api/v1/organisations?page=1&perPage=2"
	req := createRequest(http.MethodGet, url, key, nil)

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

	orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB)
	_, err = orgRepo.FetchOrganisationByID(context.Background(), uid)
	require.Equal(s.T(), datastore.ErrOrgNotFound, err)
}

func TestOrganisationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationIntegrationTestSuite))
}
