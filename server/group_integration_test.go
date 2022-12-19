//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type GroupIntegrationTestSuite struct {
	suite.Suite
	DB              cm.Client
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultGroup    *datastore.Group
	DefaultUser     *datastore.User
}

func (s *GroupIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *GroupIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.Store)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.Store, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Group.
	s.DefaultGroup, err = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, s.DefaultOrg.UID)
	require.NoError(s.T(), err)

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

func (s *GroupIntegrationTestSuite) TestGetGroup() {
	groupID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	group, err := testdb.SeedGroup(s.ConvoyApp.A.Store, groupID, "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	require.NoError(s.T(), err)
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, group, uuid.NewString(), "test-app", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.Store, endpoint, group.UID, uuid.NewString(), "*", "", []byte("{}"))

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, group.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respGroup datastore.Group
	parseResponse(s.T(), w.Result(), &respGroup)
	require.Equal(s.T(), group.UID, respGroup.UID)
	require.Equal(s.T(), datastore.GroupStatistics{
		MessagesSent: 1,
		TotalApps:    1,
	}, *respGroup.Statistics)
}

func (s *GroupIntegrationTestSuite) TestGetGroupWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultGroup.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respGroup datastore.Group
	parseResponse(s.T(), w.Result(), &respGroup)

	require.Equal(s.T(), s.DefaultGroup.UID, respGroup.UID)
	require.Equal(s.T(), s.DefaultGroup.Name, respGroup.Name)
}

func (s *GroupIntegrationTestSuite) TestGetGroupWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.Store, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultGroup.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *GroupIntegrationTestSuite) TestGetGroup_GroupNotFound() {
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, uuid.NewString())
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *GroupIntegrationTestSuite) TestDeleteGroup() {
	groupID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	group, err := testdb.SeedGroup(s.ConvoyApp.A.Store, groupID, "", "", datastore.OutgoingGroup, nil)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, group.UID)
	req := createRequest(http.MethodDelete, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
	groupRepo := cm.NewGroupRepo(s.ConvoyApp.A.Store)
	_, err = groupRepo.FetchGroupByID(context.Background(), group.UID)
	require.Equal(s.T(), datastore.ErrGroupNotFound, err)
}

func (s *GroupIntegrationTestSuite) TestDeleteGroup_GroupNotFound() {
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, uuid.NewString())
	req := createRequest(http.MethodDelete, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *GroupIntegrationTestSuite) TestDeleteGroupWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK
	groupID := uuid.NewString()

	// Just Before.
	group, err := testdb.SeedGroup(s.ConvoyApp.A.Store, groupID, "test", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", group.UID)
	req := createRequest(http.MethodDelete, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	groupRepo := cm.NewGroupRepo(s.ConvoyApp.A.Store)
	_, err = groupRepo.FetchGroupByID(context.Background(), groupID)
	require.Equal(s.T(), datastore.ErrGroupNotFound, err)
}

func (s *GroupIntegrationTestSuite) TestDeleteGroupWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.Store, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultGroup.UID)
	req := createRequest(http.MethodDelete, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *GroupIntegrationTestSuite) TestCreateGroup() {
	expectedStatusCode := http.StatusCreated

	bodyStr := `{
    "name": "test-group",
	"type": "outgoing",
    "logo_url": "",
    "config": {
        "strategy": {
            "type": "linear",
            "duration": 10,
            "retry_count": 2
        },
        "signature": {
            "header": "X-Convoy-Signature",
            "hash": "SHA512"
        },
        "disable_endpoint": false,
        "replay_attacks": false
    },
    "rate_limit": 5000,
    "rate_limit_duration": "1m"
}`

	body := serialize(bodyStr)
	url := fmt.Sprintf("/ui/organisations/%s/projects", s.DefaultOrg.UID)

	req := createRequest(http.MethodPost, url, "", body)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respGroup models.CreateGroupResponse
	parseResponse(s.T(), w.Result(), &respGroup)
	require.NotEmpty(s.T(), respGroup.Group.UID)
	require.Equal(s.T(), 5000, respGroup.Group.RateLimit)
	require.Equal(s.T(), "1m", respGroup.Group.RateLimitDuration)
	require.Equal(s.T(), "test-group", respGroup.Group.Name)
	require.Equal(s.T(), "test-group's default key", respGroup.APIKey.Name)

	require.Equal(s.T(), auth.RoleAdmin, respGroup.APIKey.Role.Type)
	require.Equal(s.T(), respGroup.Group.UID, respGroup.APIKey.Role.Group)
	require.Equal(s.T(), "test-group's default key", respGroup.APIKey.Name)
	require.NotEmpty(s.T(), respGroup.APIKey.Key)
}

func (s *GroupIntegrationTestSuite) TestCreateGroupWithPersonalAPIKey() {
	expectedStatusCode := http.StatusCreated

	bodyStr := `{
    "name": "test-group",
	"type": "outgoing",
    "logo_url": "",
    "config": {
        "strategy": {
            "type": "linear",
            "duration": 10,
            "retry_count": 2
        },
        "signature": {
            "header": "X-Convoy-Signature",
            "hash": "SHA512"
        },
        "disable_endpoint": false,
        "replay_attacks": false
    },
    "rate_limit": 5000,
    "rate_limit_duration": "1m"
}`

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	body := serialize(bodyStr)

	req := createRequest(http.MethodPost, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respGroup models.CreateGroupResponse
	parseResponse(s.T(), w.Result(), &respGroup)
	require.NotEmpty(s.T(), respGroup.Group.UID)
	require.Equal(s.T(), 5000, respGroup.Group.RateLimit)
	require.Equal(s.T(), "1m", respGroup.Group.RateLimitDuration)
	require.Equal(s.T(), "test-group", respGroup.Group.Name)
	require.Equal(s.T(), "test-group's default key", respGroup.APIKey.Name)

	require.Equal(s.T(), auth.RoleAdmin, respGroup.APIKey.Role.Type)
	require.Equal(s.T(), respGroup.Group.UID, respGroup.APIKey.Role.Group)
	require.Equal(s.T(), "test-group's default key", respGroup.APIKey.Name)
	require.NotEmpty(s.T(), respGroup.APIKey.Key)
}

func (s *GroupIntegrationTestSuite) TestCreateGroupWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.Store, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	req := createRequest(http.MethodPost, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *GroupIntegrationTestSuite) TestUpdateGroup() {
	groupID := uuid.NewString()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	group, err := testdb.SeedGroup(s.ConvoyApp.A.Store, groupID, "", "test-group", datastore.OutgoingGroup, nil)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, group.UID)

	bodyStr := `{
    "name": "group_1",
	"type": "outgoing",
    "config": {
        "strategy": {
            "type": "exponential",
            "duration": 10,
            "retry_count": 2
        },
        "signature": {
            "header": "X-Convoy-Signature",
            "hash": "SHA512"
        },
        "disable_endpoint": false,
        "replay_attacks": false
    }
}`
	req := createRequest(http.MethodPut, url, "", serialize(bodyStr))
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
	groupRepo := cm.NewGroupRepo(s.ConvoyApp.A.Store)
	g, err := groupRepo.FetchGroupByID(context.Background(), group.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "group_1", g.Name)
}

func (s *GroupIntegrationTestSuite) TestUpdateGroupWithPersonalAPIKey() {
	expectedStatusCode := http.StatusAccepted
	groupID := uuid.NewString()

	// Just Before.
	group, err := testdb.SeedGroup(s.ConvoyApp.A.Store, groupID, "test", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	body := serialize(`{"name":"update_group"}`)
	url := fmt.Sprintf("/api/v1/projects/%s", group.UID)
	req := createRequest(http.MethodPut, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respGroup datastore.Group
	parseResponse(s.T(), w.Result(), &respGroup)

	require.Equal(s.T(), groupID, respGroup.UID)
	require.Equal(s.T(), "update_group", respGroup.Name)
}

func (s *GroupIntegrationTestSuite) TestUpdateGroupWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.Store, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.Store, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultGroup.UID)
	req := createRequest(http.MethodPut, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *GroupIntegrationTestSuite) TestGetGroups() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	group1, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	group2, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	group3, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)

	url := fmt.Sprintf("/ui/organisations/%s/projects", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var groups []*datastore.Group
	parseResponse(s.T(), w.Result(), &groups)
	require.Equal(s.T(), 4, len(groups))

	v := []string{groups[0].UID, groups[1].UID, groups[2].UID, groups[3].UID}
	require.Contains(s.T(), v, group1.UID)
	require.Contains(s.T(), v, group2.UID)
	require.Contains(s.T(), v, group3.UID)
	require.Contains(s.T(), v, s.DefaultGroup.UID)
}

func (s *GroupIntegrationTestSuite) TestGetGroupsWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	group1, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	group2, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.Store, auth.Role{}, uuid.NewString(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var groups []*datastore.Group
	parseResponse(s.T(), w.Result(), &groups)
	require.Equal(s.T(), 3, len(groups))

	v := []string{groups[0].UID, groups[1].UID, groups[2].UID}
	require.Contains(s.T(), v, group1.UID)
	require.Contains(s.T(), v, group2.UID)
	require.Contains(s.T(), v, s.DefaultGroup.UID)
}

func (s *GroupIntegrationTestSuite) TestGetGroups_FilterByName() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	group1, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "abcdef", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	_, _ = testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "test-group-2", "", datastore.OutgoingGroup, nil)
	_, _ = testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "test-group-3", "", datastore.OutgoingGroup, nil)

	url := fmt.Sprintf("/ui/organisations/%s/projects?name=%s", s.DefaultOrg.UID, group1.Name)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var groups []*datastore.Group
	parseResponse(s.T(), w.Result(), &groups)
	require.Equal(s.T(), 1, len(groups))

	require.Equal(s.T(), group1.UID, groups[0].UID)
}

func (s *GroupIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func TestGroupIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(GroupIntegrationTestSuite))
}
