//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/auth"
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

type OrganisationMemberIntegrationTestSuite struct {
	suite.Suite
	DB              datastore.DatabaseClient
	Router          http.Handler
	ConvoyApp       *applicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultGroup    *datastore.Group
	DefaultUser     *datastore.User
}

func (s *OrganisationMemberIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *OrganisationMemberIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)
	s.DB = getDB()

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB, "")

	user, err := testdb.SeedDefaultUser(s.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)
}

func (s *OrganisationMemberIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *OrganisationMemberIntegrationTestSuite) Test_GetOrganisationMembers() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.DB, s.DefaultOrg, user, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	})
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, nil)

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
}

func (s *OrganisationMemberIntegrationTestSuite) Test_GetOrganisationMember() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	member, err := testdb.SeedOrganisationMember(s.DB, s.DefaultOrg, user, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	})

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members/%s", s.DefaultOrg.UID, member.UID)
	req := createRequest(http.MethodGet, url, nil)

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
}

func (s *OrganisationMemberIntegrationTestSuite) Test_UpdateOrganisationMember() {
	expectedStatusCode := http.StatusAccepted

	user, err := testdb.SeedUser(s.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	member, err := testdb.SeedOrganisationMember(s.DB, s.DefaultOrg, user, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	})
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members/%s", s.DefaultOrg.UID, member.UID)

	body := strings.NewReader(`{"role":{ "type":"api", "groups":["123"]}}`)
	req := createRequest(http.MethodPut, url, body)

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
	require.Equal(s.T(), auth.Role{Type: auth.RoleAPI, Groups: []string{"123"}}, m.Role)
}

func (s *OrganisationMemberIntegrationTestSuite) Test_DeleteOrganisationMember() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.DB, "member@test.com", "password")
	require.NoError(s.T(), err)

	member, err := testdb.SeedOrganisationMember(s.DB, s.DefaultOrg, user, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	})

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/members/%s", s.DefaultOrg.UID, member.UID)
	req := createRequest(http.MethodDelete, url, nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	_, err = s.DB.OrganisationMemberRepo().FetchOrganisationMemberByID(context.Background(), member.UID, s.DefaultOrg.UID)
	require.Equal(s.T(), datastore.ErrOrgMemberNotFound, err)
}

func TestOrganisationMemberIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationMemberIntegrationTestSuite))
}
