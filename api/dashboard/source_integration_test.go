//go:build integration
// +build integration

package dashboard

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

type SourceIntegrationTestSuite struct {
	suite.Suite
	DB             database.Database
	Router         http.Handler
	ConvoyApp      *DashboardHandler
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

	_, s.APIKey, _ = testdb.SeedAPIKey(s.ConvoyApp.A.DB, role, "", "test", "", "")

	// Setup Config.
	err = config.LoadConfig("../testdata/Auth_Config/full-convoy.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	// orgRepo := postgres.NewOrgRepo(s.ConvoyApp.A.DB)
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
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil)

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

	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB)
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
		_, err := testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, "", "", "", nil)
		require.NoError(s.T(), err)
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
	require.Equal(s.T(), totalSources, len(resp.Content.([]interface{})))
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
	sourceID := ulid.Make().String()

	// Just Before
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil)

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

	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB)
	dbSource, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), source.UID, dbSource.UID)
	require.Equal(s.T(), name, dbSource.Name)
	require.Equal(s.T(), !isDisabled, dbSource.IsDisabled)
}

func (s *SourceIntegrationTestSuite) Test_DeleteSource() {
	sourceID := ulid.Make().String()

	// Just Before.
	_, _ = testdb.SeedSource(s.ConvoyApp.A.DB, s.DefaultProject, sourceID, "", "", nil)

	// Arrange Request.
	url := fmt.Sprintf("/api/v1/projects/%s/sources/%s", s.DefaultProject.UID, sourceID)
	req := createRequest(http.MethodDelete, url, s.APIKey, nil)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), http.StatusOK, w.Code)

	// Deep Assert.
	sourceRepo := postgres.NewSourceRepo(s.ConvoyApp.A.DB)
	_, err := sourceRepo.FindSourceByID(context.Background(), s.DefaultProject.UID, sourceID)
	require.ErrorIs(s.T(), err, datastore.ErrSourceNotFound)
}

func TestSourceIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(SourceIntegrationTestSuite))
}
