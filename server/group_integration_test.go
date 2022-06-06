//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type GroupIntegrationTestSuite struct {
	suite.Suite
	DB           datastore.DatabaseClient
	Router       http.Handler
	ConvoyApp    *applicationHandler
	DefaultGroup *datastore.Group
}

func (s *GroupIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildApplication()
	s.Router = buildRoutes(s.ConvoyApp)
}

func (s *GroupIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.DB)

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	initRealmChain(s.T(), s.DB.APIRepo(), s.DB.UserRepo(), s.ConvoyApp.cache)
}

func (s *GroupIntegrationTestSuite) TestGetGroup() {
	groupID := uuid.NewString()
	expectedStatusCode := http.StatusOK

	// Just Before.
	group, err := testdb.SeedGroup(s.DB, groupID, "", nil)
	require.NoError(s.T(), err)
	app, _ := testdb.SeedApplication(s.DB, group, uuid.NewString(), "test-app", false)
	_, _ = testdb.SeedEndpoint(s.DB, app, group.UID)
	_, _ = testdb.SeedEvent(s.DB, app, group.UID, uuid.NewString(), "*", []byte("{}"))

	url := fmt.Sprintf("/api/v1/groups/%s", group.UID)
	req := createRequest(http.MethodGet, url, nil)
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

	url := fmt.Sprintf("/api/v1/groups/%s", uuid.NewString())
	req := createRequest(http.MethodGet, url, nil)
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
	group, err := testdb.SeedGroup(s.DB, groupID, "", nil)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/groups/%s", group.UID)
	req := createRequest(http.MethodDelete, url, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
	g, err := s.DB.GroupRepo().FetchGroupByID(context.Background(), group.UID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), datastore.DeletedDocumentStatus, g.DocumentStatus)
	require.True(s.T(), g.DeletedAt > 0)
}

func (s *GroupIntegrationTestSuite) TestDeleteGroup_GroupNotFound() {
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/api/v1/groups/%s", uuid.NewString())
	req := createRequest(http.MethodDelete, url, nil)
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
	req := createRequest(http.MethodPost, "/api/v1/groups", body)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respGroup datastore.Group
	parseResponse(s.T(), w.Result(), &respGroup)
	require.NotEmpty(s.T(), respGroup.UID)
	require.Equal(s.T(), 5000, respGroup.RateLimit)
	require.Equal(s.T(), "1m", respGroup.RateLimitDuration)
	require.Equal(s.T(), "test-group", respGroup.Name)
}

func (s *GroupIntegrationTestSuite) TestUpdateGroup() {
	groupID := uuid.NewString()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	group, err := testdb.SeedGroup(s.DB, groupID, "test-group", nil)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/groups/%s", group.UID)

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
	req := createRequest(http.MethodPut, url, serialize(bodyStr))
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
	g, err := s.DB.GroupRepo().FetchGroupByID(context.Background(), group.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "group_1", g.Name)
}

func (s *GroupIntegrationTestSuite) TestGetGroups() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	group1, _ := testdb.SeedGroup(s.DB, uuid.NewString(), "test-group-1", nil)
	group2, _ := testdb.SeedGroup(s.DB, uuid.NewString(), "test-group-2", nil)
	group3, _ := testdb.SeedGroup(s.DB, uuid.NewString(), "test-group-3", nil)

	req := createRequest(http.MethodGet, "/api/v1/groups", nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var groups []*datastore.Group
	parseResponse(s.T(), w.Result(), &groups)
	require.Equal(s.T(), 4, len(groups))

	v := []*datastore.Group{s.DefaultGroup, group1, group2, group3}
	for i, group := range groups {
		require.Equal(s.T(), v[i].UID, group.UID)
	}
}

func (s *GroupIntegrationTestSuite) TestGetGroups_FilterByName() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	group1, _ := testdb.SeedGroup(s.DB, uuid.NewString(), "abcdef", nil)
	_, _ = testdb.SeedGroup(s.DB, uuid.NewString(), "test-group-2", nil)
	_, _ = testdb.SeedGroup(s.DB, uuid.NewString(), "test-group-3", nil)

	url := fmt.Sprintf("/api/v1/groups?name=%s", group1.Name)
	req := createRequest(http.MethodGet, url, nil)
	req.SetBasicAuth("test-group-filter", "test-group-filter") // override previous auth in createRequest
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var groups []datastore.Group
	parseResponse(s.T(), w.Result(), &groups)
	require.Equal(s.T(), 1, len(groups))

	require.Equal(s.T(), group1.UID, groups[0].UID)
}

func (s *GroupIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.DB)
}

func TestGroupIntegrationTestSuiteTest(t *testing.T) {
	suite.Run(t, new(GroupIntegrationTestSuite))
}
