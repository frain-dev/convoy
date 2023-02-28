//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

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
	s.Router = s.ConvoyApp.BuildRoutes()
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

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
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
	require.Equal(s.T(), int64(2), pagedResp.Pagination.Total)

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

	orgMemberRepo := postgres.NewOrgMemberRepo(s.ConvoyApp.A.DB)
	_, err = orgMemberRepo.FetchOrganisationMemberByID(context.Background(), member.UID, s.DefaultOrg.UID)
	require.Equal(s.T(), datastore.ErrOrgMemberNotFound, err)
}

func (s *OrganisationMemberIntegrationTestSuite) Test_CannotDeleteOrganisationOwner() {
	expectedStatusCode := http.StatusForbidden

	orgMemberRepo := postgres.NewOrgMemberRepo(s.ConvoyApp.A.DB)
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
