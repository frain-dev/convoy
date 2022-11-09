//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PortalLinkIntegrationTestSuite struct {
	suite.Suite
	DB             cm.Client
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultGroup   *datastore.Group
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *PortalLinkIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *PortalLinkIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	// Setup Default Group.
	s.DefaultGroup, _ = testdb.SeedDefaultGroup(s.ConvoyApp.A.Store, "")

	// Seed Auth
	role := auth.Role{
		Type:  auth.RoleAdmin,
		Group: s.DefaultGroup.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	// Setup Config.
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalLinkIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PortalLinkIntegrationTestSuite) Test_CreatePortalLink() {
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), false)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), false)
	expectedStatusCode := http.StatusCreated

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links", s.DefaultGroup.UID)
	plainBody := fmt.Sprintf(`{
		"endpoints": ["%s", "%s"]
	}`, endpoint1.UID, endpoint2.UID)
	body := strings.NewReader(plainBody)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), expectedStatusCode, w.Code)

	// Deep Assert.
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.GroupID, resp.UID)
	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal/%s", pl.Token))
	require.Equal(s.T(), resp.Endpoints, pl.Endpoints)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_PortalLinkNotFound() {
	portalLinkID := "123"

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultGroup.ID, portalLinkID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinkByID_ValidPortalLink() {
	// Just Before
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), false)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint.UID})

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultGroup.UID, portalLink.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.GroupID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal/%s", pl.Token))
	require.Equal(s.T(), resp.Endpoints, pl.Endpoints)
}

func (s *PortalLinkIntegrationTestSuite) Test_GetPortalLinks_ValidPortalLinks() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalLinks := r.Intn(5)

	// Just Before
	for i := 0; i < totalLinks; i++ {
		_, _ = testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{uuid.NewString()})
	}

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links", s.DefaultGroup.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalLinks), resp.Pagination.Total)
}

func (s *PortalLinkIntegrationTestSuite) Test_UpdatePortalLinks() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), false)
	endpoint2, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), false)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint1.UID})

	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s", s.DefaultGroup.UID, portalLink.UID)
	bodyStr := fmt.Sprintf(`{
			"endpoints": ["%s"]
		}`, endpoint2.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Assert
	var resp models.PortalLinkResponse
	parseResponse(s.T(), w.Result(), &resp)

	portalLinkRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	pl, err := portalLinkRepo.FindPortalLinkByID(context.Background(), resp.GroupID, resp.UID)

	require.NoError(s.T(), err)

	require.Equal(s.T(), resp.UID, pl.UID)
	require.Equal(s.T(), resp.URL, fmt.Sprintf("https://app.convoy.io/portal/%s", pl.Token))
	require.Equal(s.T(), resp.Endpoints, pl.Endpoints)
}

func (s *PortalLinkIntegrationTestSuite) Test_RevokePortalLink() {
	// Just Before
	endpoint1, _ := testdb.SeedEndpoint(s.ConvoyApp.A.Store, s.DefaultGroup, "", uuid.NewString(), false)
	portalLink, _ := testdb.SeedPortalLink(s.ConvoyApp.A.Store, s.DefaultGroup, []string{endpoint1.UID})

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/portal-links/%s/revoke", s.DefaultGroup.UID, portalLink.UID)
	req := createRequest(http.MethodPut, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	plRepo := cm.NewPortalLinkRepo(s.ConvoyApp.A.Store)
	_, err := plRepo.FindPortalLinkByID(context.Background(), s.DefaultGroup.UID, portalLink.UID)
	require.ErrorIs(s.T(), err, datastore.ErrPortalLinkNotFound)
}

func TestPortalLinkIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(PortalLinkIntegrationTestSuite))
}
