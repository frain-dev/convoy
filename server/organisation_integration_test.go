//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OrganisationIntegrationTestSuite struct {
	suite.Suite
	DB              cm.Client
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

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.Store, "")

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.Store)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.Store, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-all-realms.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
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

	orgRepo := cm.NewOrgRepo(s.ConvoyApp.A.Store)
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

	uid := uuid.NewString()
	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.Store, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, org, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
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
	require.Equal(s.T(), "abc.com", organisation.CustomDomain)
}

func (s *OrganisationIntegrationTestSuite) Test_UpdateOrganisation() {
	expectedStatusCode := http.StatusAccepted

	uid := uuid.NewString()
	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.Store, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, org, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
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

	orgRepo := cm.NewOrgRepo(s.ConvoyApp.A.Store)
	organisation, err := orgRepo.FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "update_org", organisation.Name)
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisation() {
	expectedStatusCode := http.StatusOK

	uid := uuid.NewString()
	seedOrg, err := testdb.SeedOrganisation(s.ConvoyApp.A.Store, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, seedOrg, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
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

	orgRepo := cm.NewOrgRepo(s.ConvoyApp.A.Store)
	org, err := orgRepo.FetchOrganisationByID(context.Background(), uid)
	require.NoError(s.T(), err)
	require.Equal(s.T(), seedOrg.Name, org.Name)
	require.Equal(s.T(), seedOrg.UID, organisation.UID)
}

func (s *OrganisationIntegrationTestSuite) Test_GetOrganisations() {
	expectedStatusCode := http.StatusOK

	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.Store, uuid.NewString(), "", "test-org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, org, s.DefaultUser, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  uuid.NewString(),
		Endpoint: "",
	})
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

	org, err := testdb.SeedOrganisation(s.ConvoyApp.A.Store, uuid.NewString(), "", "test-org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, org, s.DefaultUser, &auth.Role{
		Type:     auth.RoleAdmin,
		Project:  uuid.NewString(),
		Endpoint: "",
	})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
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

	uid := uuid.NewString()
	seedOrg, err := testdb.SeedOrganisation(s.ConvoyApp.A.Store, uid, s.DefaultUser.UID, "new_org")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, seedOrg, s.DefaultUser, &auth.Role{Type: auth.RoleSuperUser})
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

	orgRepo := cm.NewOrgRepo(s.ConvoyApp.A.Store)
	_, err = orgRepo.FetchOrganisationByID(context.Background(), uid)
	require.Equal(s.T(), datastore.ErrOrgNotFound, err)
}

func TestOrganisationIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationIntegrationTestSuite))
}
