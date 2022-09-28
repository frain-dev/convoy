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
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, "")

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
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-jwt-realm.json")
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
	app, _ := testdb.SeedApplication(s.ConvoyApp.A.Store, group, uuid.NewString(), "test-app", false)
	_, _ = testdb.SeedEndpoint(s.ConvoyApp.A.Store, app, group.UID)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.Store, app, group.UID, uuid.NewString(), "*", "", []byte("{}"))

	url := fmt.Sprintf("/ui/organisations/%s/groups/%s", s.DefaultOrg.UID, group.UID)
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

func (s *GroupIntegrationTestSuite) TestGetGroup_GroupNotFound() {
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/ui/organisations/%s/groups/%s", s.DefaultOrg.UID, uuid.NewString())
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

	url := fmt.Sprintf("/ui/organisations/%s/groups/%s", s.DefaultOrg.UID, group.UID)
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

	url := fmt.Sprintf("/ui/organisations/%s/groups/%s", s.DefaultOrg.UID, uuid.NewString())
	req := createRequest(http.MethodDelete, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
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
	url := fmt.Sprintf("/ui/organisations/%s/groups", s.DefaultOrg.UID)

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

func (s *GroupIntegrationTestSuite) TestUpdateGroup() {
	groupID := uuid.NewString()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	group, err := testdb.SeedGroup(s.ConvoyApp.A.Store, groupID, "", "test-group", datastore.OutgoingGroup, nil)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/groups/%s", s.DefaultOrg.UID, group.UID)

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

func (s *GroupIntegrationTestSuite) TestGetGroups() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	group1, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	group2, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	group3, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)

	url := fmt.Sprintf("/ui/organisations/%s/groups", s.DefaultOrg.UID)
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
	require.Equal(s.T(), 3, len(groups))

	v := []string{group1.UID, group2.UID, group3.UID}
	require.Contains(s.T(), v, group1.UID)
	require.Contains(s.T(), v, group2.UID)
	require.Contains(s.T(), v, group3.UID)
}

func (s *GroupIntegrationTestSuite) TestGetGroups_FilterByName() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	group1, _ := testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "abcdef", s.DefaultOrg.UID, datastore.OutgoingGroup, nil)
	_, _ = testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "test-group-2", "", datastore.OutgoingGroup, nil)
	_, _ = testdb.SeedGroup(s.ConvoyApp.A.Store, uuid.NewString(), "test-group-3", "", datastore.OutgoingGroup, nil)

	url := fmt.Sprintf("/ui/organisations/%s/groups?name=%s", s.DefaultOrg.UID, group1.Name)
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
	testdb.PurgeDB(s.DB)
	metrics.Reset()
}

func TestGroupIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(GroupIntegrationTestSuite))
}
