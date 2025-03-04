//go:build integration
// +build integration

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PortalTokenTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
	PersonalAPIKey string
}

func (s *PortalTokenTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *PortalTokenTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.DB)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.DB, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, err = testdb.SeedDefaultProject(s.ConvoyApp.A.DB, s.DefaultOrg.UID)
	require.NoError(s.T(), err)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")
	require.NoError(s.T(), err)

	_, s.PersonalAPIKey, err = testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test-personal-key", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	portalLinkRepo := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *PortalTokenTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *PortalTokenTestSuite) Test_GetEndpoint_WithTokenOnly() {
	// Create an endpoint to associate with the portal link
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	// Create a portal link with the endpoint
	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	// Create a request to get the endpoint using only token
	url := fmt.Sprintf("/portal-api/endpoints/%s?token=%s", endpoint.UID, portalLink.Token)
	fmt.Println("Request URL:", url)
	fmt.Println("Endpoint UID:", endpoint.UID)
	fmt.Println("Token:", portalLink.Token)

	// Create a request with the Authorization header set to the token
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", portalLink.Token))

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Print response body for debugging
	fmt.Println("Response body:", w.Body.String())

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp struct {
		Message string             `json:"message"`
		Data    datastore.Endpoint `json:"data"`
	}
	err = json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&resp)
	require.NoError(s.T(), err)

	// Verify the endpoint details
	require.Equal(s.T(), endpoint.UID, resp.Data.UID)
}

func (s *PortalTokenTestSuite) Test_GetEndpoint_WithOwnerID() {
	// Create a portal link with owner_id
	ownerID := "test-owner-id"

	// Create an endpoint to associate with the portal link
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	// Create a portal link with the endpoint
	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	// Update the portal link with owner_id
	portalLink.OwnerID = ownerID
	err = postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB).UpdatePortalLink(context.Background(), s.DefaultProject.UID, portalLink)
	require.NoError(s.T(), err)

	// Verify the portal link was updated correctly
	updatedPortalLink, err := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB).FindPortalLinkByID(context.Background(), s.DefaultProject.UID, portalLink.UID)
	require.NoError(s.T(), err)
	fmt.Println("Updated Portal Link:", updatedPortalLink)
	fmt.Println("Updated Portal Link Owner ID:", updatedPortalLink.OwnerID)

	// Create a request to get the endpoint using token in Authorization header and owner_id in query
	url := fmt.Sprintf("/portal-api/endpoints/%s?owner_id=%s", endpoint.UID, ownerID)
	fmt.Println("Request URL:", url)
	fmt.Println("Owner ID:", ownerID)
	fmt.Println("Endpoint UID:", endpoint.UID)

	// Create a request with the Authorization header set to the token
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", portalLink.Token))

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Print response body for debugging
	fmt.Println("Response body:", w.Body.String())

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp struct {
		Message string             `json:"message"`
		Data    datastore.Endpoint `json:"data"`
	}
	err = json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&resp)
	require.NoError(s.T(), err)

	// Verify the endpoint details
	require.Equal(s.T(), endpoint.UID, resp.Data.UID)
}

func (s *PortalTokenTestSuite) Test_GetPortalLink_WithOwnerID() {
	// Create a portal link with owner_id
	ownerID := "test-owner-id"

	// Create an endpoint to associate with the portal link
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	// Create a portal link with the endpoint
	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	// Update the portal link with owner_id
	portalLink.OwnerID = ownerID
	err = postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB).UpdatePortalLink(context.Background(), s.DefaultProject.UID, portalLink)
	require.NoError(s.T(), err)

	// Verify the portal link was updated correctly
	updatedPortalLink, err := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB).FindPortalLinkByID(context.Background(), s.DefaultProject.UID, portalLink.UID)
	require.NoError(s.T(), err)
	fmt.Println("Updated Portal Link:", updatedPortalLink)
	fmt.Println("Updated Portal Link Owner ID:", updatedPortalLink.OwnerID)

	// Create a request to get the portal link using token in Authorization header and owner_id in query
	url := fmt.Sprintf("/portal-api/portal_link?owner_id=%s", ownerID)
	fmt.Println("Request URL:", url)
	fmt.Println("Owner ID:", ownerID)

	// Create a request with the Authorization header set to the token
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", portalLink.Token))

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Print response body for debugging
	fmt.Println("Response body:", w.Body.String())

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp struct {
		Message string               `json:"message"`
		Data    datastore.PortalLink `json:"data"`
	}
	err = json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&resp)
	require.NoError(s.T(), err)

	// Verify the portal link details
	require.Equal(s.T(), portalLink.UID, resp.Data.UID)
	require.Equal(s.T(), ownerID, resp.Data.OwnerID)
}

func (s *PortalTokenTestSuite) Test_GetPortalLink_WithTokenOnly() {
	// Create a portal link with owner_id
	ownerID := "test-owner-id"

	// Create an endpoint to associate with the portal link
	endpoint, err := testdb.SeedEndpoint(s.ConvoyApp.A.DB, s.DefaultProject, "", ulid.Make().String(), "", false, datastore.ActiveEndpointStatus)
	require.NoError(s.T(), err)

	// Create a portal link with the endpoint
	portalLink, err := testdb.SeedPortalLink(s.ConvoyApp.A.DB, s.DefaultProject, []string{endpoint.UID})
	require.NoError(s.T(), err)

	// Update the portal link with owner_id
	portalLink.OwnerID = ownerID
	err = postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB).UpdatePortalLink(context.Background(), s.DefaultProject.UID, portalLink)
	require.NoError(s.T(), err)

	// Verify the portal link was updated correctly
	updatedPortalLink, err := postgres.NewPortalLinkRepo(s.ConvoyApp.A.DB).FindPortalLinkByID(context.Background(), s.DefaultProject.UID, portalLink.UID)
	require.NoError(s.T(), err)
	fmt.Println("Updated Portal Link:", updatedPortalLink)
	fmt.Println("Updated Portal Link Owner ID:", updatedPortalLink.OwnerID)

	// Create a request to get the portal link using only token in Authorization header
	url := "/portal-api/portal_link"
	fmt.Println("Request URL:", url)
	fmt.Println("Token:", portalLink.Token)

	// Create a request with the Authorization header set to the token
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", portalLink.Token))

	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Print response body for debugging
	fmt.Println("Response body:", w.Body.String())

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp struct {
		Message string               `json:"message"`
		Data    datastore.PortalLink `json:"data"`
	}
	err = json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&resp)
	require.NoError(s.T(), err)

	// Verify the portal link details
	require.Equal(s.T(), portalLink.UID, resp.Data.UID)
	require.Equal(s.T(), ownerID, resp.Data.OwnerID)
}

func TestPortalTokenTestSuite(t *testing.T) {
	suite.Run(t, new(PortalTokenTestSuite))
}
