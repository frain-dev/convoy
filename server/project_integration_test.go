//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ProjectIntegrationTestSuite struct {
	suite.Suite
	DB              database.Database
	Router          http.Handler
	ConvoyApp       *ApplicationHandler
	AuthenticatorFn AuthenticatorFn
	DefaultOrg      *datastore.Organisation
	DefaultProject  *datastore.Project
	DefaultUser     *datastore.User
}

func (s *ProjectIntegrationTestSuite) SetupSuite() {
	s.DB = getDB()
	s.ConvoyApp = buildServer()
	s.Router = s.ConvoyApp.BuildRoutes()
}

func (s *ProjectIntegrationTestSuite) SetupTest() {
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

	s.AuthenticatorFn = authenticateRequest(&models.LoginUser{
		Username: user.Email,
		Password: testdb.DefaultUserPassword,
	})

	// Setup Config.
	err = config.LoadConfig("./testdata/Auth_Config/full-convoy-with-all-realms.json")
	require.NoError(s.T(), err)

	apiRepo := postgres.NewAPIKeyRepo(s.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, s.ConvoyApp.A.Cache)
}

func (s *ProjectIntegrationTestSuite) TestGetProject() {
	projectID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "", s.DefaultOrg.UID, datastore.OutgoingProject, nil)
	require.NoError(s.T(), err)
	endpoint, _ := testdb.SeedEndpoint(s.ConvoyApp.A.DB, project, ulid.Make().String(), "test-app", "", false, datastore.ActiveEndpointStatus)
	_, _ = testdb.SeedEvent(s.ConvoyApp.A.DB, endpoint, project.UID, ulid.Make().String(), "*", "", []byte("{}"))

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, project.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject datastore.Project
	parseResponse(s.T(), w.Result(), &respProject)
	require.Equal(s.T(), project.UID, respProject.UID)
	require.Equal(s.T(), datastore.ProjectStatistics{
		MessagesSent:   1,
		TotalEndpoints: 1,
	}, *respProject.Statistics)
}

func (s *ProjectIntegrationTestSuite) TestGetProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject datastore.Project
	parseResponse(s.T(), w.Result(), &respProject)

	require.Equal(s.T(), s.DefaultProject.UID, respProject.UID)
	require.Equal(s.T(), s.DefaultProject.Name, respProject.Name)
}

func (s *ProjectIntegrationTestSuite) TestGetProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultProject.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestGetProject_ProjectNotFound() {
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, ulid.Make().String())
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestDeleteProject() {
	projectID := ulid.Make().String()
	expectedStatusCode := http.StatusOK

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "", "", datastore.OutgoingProject, nil)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, project.UID)
	req := createRequest(http.MethodDelete, url, "", nil)
	err = s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
	projectRepo := postgres.NewProjectRepo(s.ConvoyApp.A.DB)
	_, err = projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.Equal(s.T(), datastore.ErrProjectNotFound, err)
}

func (s *ProjectIntegrationTestSuite) TestDeleteProject_ProjectNotFound() {
	expectedStatusCode := http.StatusNotFound

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, ulid.Make().String())
	req := createRequest(http.MethodDelete, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestDeleteProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK
	projectID := ulid.Make().String()

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "test", s.DefaultOrg.UID, datastore.OutgoingProject, nil)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", project.UID)
	req := createRequest(http.MethodDelete, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	projectRepo := postgres.NewProjectRepo(s.ConvoyApp.A.DB)
	_, err = projectRepo.FetchProjectByID(context.Background(), projectID)
	require.Equal(s.T(), datastore.ErrProjectNotFound, err)
}

func (s *ProjectIntegrationTestSuite) TestDeleteProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultProject.UID)
	req := createRequest(http.MethodDelete, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestCreateProject() {
	expectedStatusCode := http.StatusCreated

	bodyStr := `{
    "name": "test-project",
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
        "rate_limit": {
            "count": 8000,
            "duration": "5m"
        },
    },
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

	var respProject models.CreateProjectResponse
	parseResponse(s.T(), w.Result(), &respProject)
	require.NotEmpty(s.T(), respProject.Project.UID)
	require.Equal(s.T(), 8000, respProject.Project.Config.RateLimit.Count)
	require.Equal(s.T(), "5m", respProject.Project.Config.RateLimit.Duration)
	require.Equal(s.T(), "test-project", respProject.Project.Name)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)

	require.Equal(s.T(), auth.RoleAdmin, respProject.APIKey.Role.Type)
	require.Equal(s.T(), respProject.Project.UID, respProject.APIKey.Role.Project)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)
	require.NotEmpty(s.T(), respProject.APIKey.Key)
}

func (s *ProjectIntegrationTestSuite) TestCreateProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusCreated

	bodyStr := `{
    "name": "test-project",
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
        "rate_limit": {
            "count": 8000,
            "duration": "5m"
        },
    },
}`

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	body := serialize(bodyStr)

	req := createRequest(http.MethodPost, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject models.CreateProjectResponse
	parseResponse(s.T(), w.Result(), &respProject)
	require.NotEmpty(s.T(), respProject.Project.UID)
	require.Equal(s.T(), 8000, respProject.Project.Config.RateLimit.Count)
	require.Equal(s.T(), "5m", respProject.Project.Config.RateLimit.Duration)
	require.Equal(s.T(), "test-project", respProject.Project.Name)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)

	require.Equal(s.T(), auth.RoleAdmin, respProject.APIKey.Role.Type)
	require.Equal(s.T(), respProject.Project.UID, respProject.APIKey.Role.Project)
	require.Equal(s.T(), "test-project's default key", respProject.APIKey.Name)
	require.NotEmpty(s.T(), respProject.APIKey.Key)
}

func (s *ProjectIntegrationTestSuite) TestCreateProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	req := createRequest(http.MethodPost, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestUpdateProject() {
	projectID := ulid.Make().String()
	expectedStatusCode := http.StatusAccepted

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "", "test-project", datastore.OutgoingProject, nil)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/ui/organisations/%s/projects/%s", s.DefaultOrg.UID, project.UID)

	bodyStr := `{
    "name": "project_1",
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
	projectRepo := postgres.NewProjectRepo(s.ConvoyApp.A.DB)
	g, err := projectRepo.FetchProjectByID(context.Background(), project.UID)
	require.NoError(s.T(), err)
	require.Equal(s.T(), "project_1", g.Name)
}

func (s *ProjectIntegrationTestSuite) TestUpdateProjectWithPersonalAPIKey() {
	expectedStatusCode := http.StatusAccepted
	projectID := ulid.Make().String()

	// Just Before.
	project, err := testdb.SeedProject(s.ConvoyApp.A.DB, projectID, "test", s.DefaultOrg.UID, datastore.OutgoingProject, nil)
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	body := serialize(`{"name":"update_project"}`)
	url := fmt.Sprintf("/api/v1/projects/%s", project.UID)
	req := createRequest(http.MethodPut, url, key, body)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var respProject datastore.Project
	parseResponse(s.T(), w.Result(), &respProject)

	require.Equal(s.T(), projectID, respProject.UID)
	require.Equal(s.T(), "update_project", respProject.Name)
}

func (s *ProjectIntegrationTestSuite) TestUpdateProjectWithPersonalAPIKey_UnauthorizedRole() {
	expectedStatusCode := http.StatusUnauthorized

	user, err := testdb.SeedUser(s.ConvoyApp.A.DB, "test@gmail.com", testdb.DefaultUserPassword)
	require.NoError(s.T(), err)

	_, err = testdb.SeedOrganisationMember(s.ConvoyApp.A.DB, s.DefaultOrg, user, &auth.Role{Type: auth.RoleAPI})
	require.NoError(s.T(), err)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), user.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects/%s", s.DefaultProject.UID)
	req := createRequest(http.MethodPut, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)
}

func (s *ProjectIntegrationTestSuite) TestGetProjects() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	project1, _ := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "", s.DefaultOrg.UID, datastore.OutgoingProject, nil)
	project2, _ := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "", s.DefaultOrg.UID, datastore.OutgoingProject, nil)
	project3, _ := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "", s.DefaultOrg.UID, datastore.OutgoingProject, nil)

	url := fmt.Sprintf("/ui/organisations/%s/projects", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var projects []*datastore.Project
	parseResponse(s.T(), w.Result(), &projects)
	require.Equal(s.T(), 4, len(projects))

	v := []string{projects[0].UID, projects[1].UID, projects[2].UID, projects[3].UID}
	require.Contains(s.T(), v, project1.UID)
	require.Contains(s.T(), v, project2.UID)
	require.Contains(s.T(), v, project3.UID)
	require.Contains(s.T(), v, s.DefaultProject.UID)
}

func (s *ProjectIntegrationTestSuite) TestGetProjectsWithPersonalAPIKey() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	project1, _ := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "", s.DefaultOrg.UID, datastore.OutgoingProject, nil)
	project2, _ := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "", s.DefaultOrg.UID, datastore.OutgoingProject, nil)

	_, key, err := testdb.SeedAPIKey(s.ConvoyApp.A.DB, auth.Role{}, ulid.Make().String(), "test", string(datastore.PersonalKey), s.DefaultUser.UID)
	require.NoError(s.T(), err)

	url := fmt.Sprintf("/api/v1/projects?orgID=%s", s.DefaultOrg.UID)
	req := createRequest(http.MethodGet, url, key, nil)

	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var projects []*datastore.Project
	parseResponse(s.T(), w.Result(), &projects)
	require.Equal(s.T(), 3, len(projects))

	v := []string{projects[0].UID, projects[1].UID, projects[2].UID}
	require.Contains(s.T(), v, project1.UID)
	require.Contains(s.T(), v, project2.UID)
	require.Contains(s.T(), v, s.DefaultProject.UID)
}

func (s *ProjectIntegrationTestSuite) TestGetProjects_FilterByName() {
	expectedStatusCode := http.StatusOK

	// Just Before.
	project1, _ := testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "abcdef", s.DefaultOrg.UID, datastore.OutgoingProject, nil)
	_, _ = testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test-project-2", "", datastore.OutgoingProject, nil)
	_, _ = testdb.SeedProject(s.ConvoyApp.A.DB, ulid.Make().String(), "test-project-3", "", datastore.OutgoingProject, nil)

	url := fmt.Sprintf("/ui/organisations/%s/projects?name=%s", s.DefaultOrg.UID, project1.Name)
	req := createRequest(http.MethodGet, url, "", nil)
	err := s.AuthenticatorFn(req, s.Router)
	require.NoError(s.T(), err)
	w := httptest.NewRecorder()

	// Act.
	s.Router.ServeHTTP(w, req)

	// Assert.
	require.Equal(s.T(), expectedStatusCode, w.Code)

	var projects []*datastore.Project
	parseResponse(s.T(), w.Result(), &projects)
	require.Equal(s.T(), 1, len(projects))

	require.Equal(s.T(), project1.UID, projects[0].UID)
}

func (s *ProjectIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(s.T(), s.DB)
	metrics.Reset()
}

func TestProjectIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectIntegrationTestSuite))
}
