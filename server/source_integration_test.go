//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type SourceIntegrationTestSuite struct {
	suite.Suite
	DB             cm.Client
	Router         http.Handler
	ConvoyApp      *ApplicationHandler
	DefaultOrg     *datastore.Organisation
	DefaultProject *datastore.Project
	DefaultUser    *datastore.User
	APIKey         string
}

func (s *SourceIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *SourceIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(s.T(), s.DB)

	user, err := testdb.SeedDefaultUser(s.ConvoyApp.A.Store)
	require.NoError(s.T(), err)
	s.DefaultUser = user

	org, err := testdb.SeedDefaultOrganisation(s.ConvoyApp.A.Store, user)
	require.NoError(s.T(), err)
	s.DefaultOrg = org

	// Setup Default Project.
	s.DefaultProject, _ = testdb.SeedDefaultProject(s.ConvoyApp.A.Store, s.DefaultOrg.UID)

	// Seed Auth
	role := auth.Role{
		Type:    auth.RoleAdmin,
		Project: s.DefaultProject.UID,
	}

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.Store, role, "", "test", "", "")

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := cm.NewApiKeyRepo(s.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(s.ConvoyApp.A.Store)
	// orgRepo := cm.NewOrgRepo(s.ConvoyApp.A.Store)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *SourceIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func (s *SourceIntegrationTestSuite) Test_GetSourceByID_SourceNotFound() {
	sourceID := "123"

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusNotFound, w.Code)
}

func (s *SourceIntegrationTestSuite) Test_GetSourceBy_ValidSource() {
	sourceID := "123456789"

	// Just Before
	_, _ = testdb.SeedSource(s.ConvoyApp.A.Store, s.DefaultProject, sourceID, "", "", nil)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	sourceRepo := cm.NewSourceRepo(s.ConvoyApp.A.Store)
	dbSource, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), source.UID, dbSource.UID)
	require.Equal(s.T(), source.Name, dbSource.Name)
	require.Equal(s.T(), source.MaskID, dbSource.MaskID)
}

func (s *SourceIntegrationTestSuite) Test_GetSource_ValidSources() {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	totalSources := r.Intn(5)

	// Just Before
	for i := 0; i < totalSources; i++ {
		_, _ = testdb.SeedSource(s.ConvoyApp.A.Store, s.DefaultProject, "", "", "", nil)
	}

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert
	var resp pagedResponse
	parseResponse(s.T(), w.Result(), &resp)
	require.Equal(s.T(), int64(totalSources), resp.Pagination.Total)
}

func (s *SourceIntegrationTestSuite) Test_CreateSource() {
	bodyStr := `{
		"name": "convoy-prod",
		"type": "http",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusCreated, w.Code)

	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	require.NotEmpty(s.T(), source.UID)
	require.Equal(s.T(), "convoy-prod", source.Name)
	require.Equal(s.T(), datastore.SourceType("http"), source.Type)
	require.Equal(s.T(), datastore.VerifierType("hmac"), source.Verifier.Type)
}

func (s *SourceIntegrationTestSuite) Test_CreateSource_RedirectToProjects() {
	bodyStr := `{
		"name": "convoy-prod",
		"type": "http",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/api/v1/sources?groupID=%s", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusTemporaryRedirect, w.Code)
}

func (s *SourceIntegrationTestSuite) Test_CreateSource_NoName() {
	bodyStr := `{
		"type": "http",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)
	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SourceIntegrationTestSuite) Test_CreateSource_InvalidSourceType() {
	bodyStr := `{
		"name": "convoy-prod",
		"type": "some-random-source-type",
		"is_disabled": false,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "base64",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`

	url := fmt.Sprintf("/api/v1/projects/%s/sources", s.DefaultProject.UID)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusBadRequest, w.Code)
}

func (s *SourceIntegrationTestSuite) Test_UpdateSource() {
	name := "updated-convoy-prod"
	isDisabled := randBool()
	sourceID := uuid.New().String()

	// Just Before
	_, _ = testdb.SeedSource(s.ConvoyApp.A.Store, s.DefaultProject, sourceID, "", "", nil)

	// Arrange Request
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	bodyStr := fmt.Sprintf(`{
		"name": "%s",
		"type": "http",
		"is_disabled": %t,
		"verifier": {
			"type": "hmac",
			"hmac": {
				"encoding": "hex",
				"header": "X-Convoy-Header",
				"hash": "SHA512",
				"secret": "convoy-secret"
			}
		}
	}`, name, !isDisabled)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPut, url, s.APIKey, body)
	w := httptest.NewRecorder()

	// Act
	s.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(s.T(), http.StatusAccepted, w.Code)

	// Deep Asset
	var source datastore.Source
	parseResponse(s.T(), w.Result(), &source)

	sourceRepo := cm.NewSourceRepo(s.ConvoyApp.A.Store)
	dbSource, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), source.UID, dbSource.UID)
	require.Equal(s.T(), name, dbSource.Name)
	require.Equal(s.T(), !isDisabled, dbSource.IsDisabled)
}

func (s *SourceIntegrationTestSuite) Test_DeleteSource() {
	sourceID := uuid.New().String()

	// Just Before.
	_, _ = testdb.SeedSource(s.ConvoyApp.A.Store, s.DefaultProject, sourceID, "", "", nil)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	sourceRepo := cm.NewSourceRepo(s.ConvoyApp.A.Store)
	_, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.ErrorIs(s.T(), err, datastore.ErrSourceNotFound)
}

func TestSourceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SourceIntegrationTestSuite))
}
