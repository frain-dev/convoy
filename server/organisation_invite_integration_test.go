//go:build integration
// +build integration

package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganisationInviteIntegrationTestSuite struct {
	suite.Suite
	DB              datastore.DatabaseClient
	Router          http.Handler
	ConvoyApp       *applicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultGroup    *datastore.Group
	DefaultUser     *datastore.User
}

func (s *OrganisationInviteIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *OrganisationInviteIntegrationTestSuite) SetupTest() {
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

func (s *OrganisationInviteIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation() {
	expectedStatusCode := http.StatusCreated

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invite_user", s.DefaultOrg.UID)

	// TODO(daniel): when the generic mailer is integrated we have to mock it
	body := strings.NewReader(`{"invitee_email":"test@invite.com","role":{"type":"api", "groups":["123"]}}`)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation_InvalidRole() {
	expectedStatusCode := http.StatusBadRequest

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invite_user", s.DefaultOrg.UID)

	body := strings.NewReader(`{"invitee_email":"test@invite.com",role":{"type":"api"}}`)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation_InvalidEmail() {
	expectedStatusCode := http.StatusBadRequest

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invite_user", s.DefaultOrg.UID)

	body := strings.NewReader(`{"invitee_email":"test_invite.com",role":{"type":"api","groups":["123"]}}`)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_InviteUserToOrganisation_EmptyEmail() {
	expectedStatusCode := http.StatusBadRequest

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/invite_user", s.DefaultOrg.UID)

	body := strings.NewReader(`{"invitee_email":"",role":{"type":"api","groups":["123"]}}`)
	req := createRequest(http.MethodPost, url, "", body)

	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_GetPendingOrganisationInvites() {
	expectedStatusCode := http.StatusOK

	_, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, "invite1@test.com", &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, "invite2@test.com", &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/%s/pending_invites", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)

	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var invites []datastore.OrganisationInvite
	pagedResp := pagedResponse{Content: &invites}
	parseResponse(s.T(), w.Result(), &pagedResp)

	require.Equal(s.T(), 2, len(invites))
	require.Equal(s.T(), int64(2), pagedResp.Pagination.Total)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_AcceptForExistingUser() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.DB, "invite@test.com", "password")
	require.NoError(s.T(), err)

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, user.Email, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)
	body := strings.NewReader(`{"password":"password"}`)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_AcceptForExistingUser_EmptyPassword() {
	expectedStatusCode := http.StatusBadRequest

	user, err := testdb.SeedUser(s.DB, "invite@test.com", "password")
	require.NoError(s.T(), err)

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, user.Email, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)
	body := strings.NewReader(`{"password":""}`)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_InviteExpired() {
	expectedStatusCode := http.StatusBadRequest

	user, err := testdb.SeedUser(s.DB, "invite@test.com", "password")
	require.NoError(s.T(), err)

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, user.Email, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(-time.Minute)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)
	req := createRequest(http.MethodPost, url, "", nil)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_AcceptForNewUser() {
	expectedStatusCode := http.StatusOK

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, "test@invite.com", &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)

	body := strings.NewReader(`{"first_name":"test","last_name":"test","email":"test@invite.com","password":"password"}`)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_EmptyFirstName() {
	expectedStatusCode := http.StatusBadRequest

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, "test@invite.com", &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=true", iv.Token)

	body := strings.NewReader(`{"first_name":"","last_name":"test","email":"test@invite.com","password":"password"}`)
	req := createRequest(http.MethodPost, url, "", body)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_ProcessOrganisationMemberInvite_Decline() {
	expectedStatusCode := http.StatusOK

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, "test@invite.com", &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/organisations/process_invite?token=%s&accepted=false", iv.Token)
	req := createRequest(http.MethodPost, url, "", nil)
	req.Header.Set("Authorization", "")

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_FindUserByInviteToken_ExistingUser() {
	expectedStatusCode := http.StatusOK

	user, err := testdb.SeedUser(s.DB, "invite@test.com", "password")
	require.NoError(s.T(), err)

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, user.Email, &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/users/token?token=%s", iv.Token)
	req := createRequest(http.MethodGet, url, "", nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response models.UserInviteTokenResponse
	parseResponse(s.T(), w.Result(), &response)

	require.Equal(s.T(), user.UID, response.User.UID)
	require.Equal(s.T(), user.FirstName, response.User.FirstName)
	require.Equal(s.T(), user.LastName, response.User.LastName)
	require.Equal(s.T(), user.Email, response.User.Email)
	require.Equal(s.T(), iv.UID, response.Token.UID)
	require.Equal(s.T(), iv.InviteeEmail, response.Token.InviteeEmail)
	require.Equal(s.T(), iv.Token, response.Token.Token)
}

func (s *OrganisationInviteIntegrationTestSuite) Test_FindUserByInviteToken_NewUser() {
	expectedStatusCode := http.StatusOK

	iv, err := testdb.SeedOrganisationInvite(s.DB, s.DefaultOrg, "invite@test.com", &auth.Role{
		Type:   auth.RoleAdmin,
		Groups: []string{uuid.NewString()},
		Apps:   nil,
	}, primitive.NewDateTimeFromTime(time.Now().Add(time.Hour)), datastore.InviteStatusPending)
	require.NoError(s.T(), err)

	// Arrange.
	url := fmt.Sprintf("/ui/users/token?token=%s", iv.Token)
	req := createRequest(http.MethodGet, url, "", nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var response models.UserInviteTokenResponse
	parseResponse(s.T(), w.Result(), &response)

	require.Equal(s.T(), iv.UID, response.Token.UID)
	require.Equal(s.T(), iv.InviteeEmail, response.Token.InviteeEmail)
	require.Equal(s.T(), iv.Token, response.Token.Token)
	require.Nil(s.T(), response.User)
}

func TestOrganisationInviteIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OrganisationInviteIntegrationTestSuite))
}
