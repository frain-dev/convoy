//go:build integration
// +build integration

package dashboard

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/testdb"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/metrics"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AuthIntegrationTestSuite struct {
	suite.Suite
	DB        database.Database
	Router    http.Handler
	ConvoyApp *DashboardHandler
	jwt       *jwt.Jwt
}

func (u *AuthIntegrationTestSuite) SetupSuite() {
	u.DB = getDB()
	u.ConvoyApp = buildServer()
	u.Router = u.ConvoyApp.BuildRoutes()
}

func (u *AuthIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(u.T(), u.DB)

	err := config.LoadConfig("../testdata/Auth_Config/jwt-convoy.json")
	require.NoError(u.T(), err)

	config, err := config.Get()
	require.NoError(u.T(), err)

	u.jwt = jwt.NewJwt(&config.Auth.Jwt, u.ConvoyApp.A.Cache)

	apiRepo := postgres.NewAPIKeyRepo(u.ConvoyApp.A.DB)
	userRepo := postgres.NewUserRepo(u.ConvoyApp.A.DB)
	initRealmChain(u.T(), apiRepo, userRepo, u.ConvoyApp.A.Cache)
}

func (u *AuthIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(u.T(), u.DB)
	metrics.Reset()
}

func (u *AuthIntegrationTestSuite) Test_LoginUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	// Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
		"username": "%s",
		"password": "%s"
	}`, user.Email, password)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response models.LoginUserResponse
	parseResponse(u.T(), w.Result(), &response)

	require.NotEmpty(u.T(), response.UID)
	require.NotEmpty(u.T(), response.Token.AccessToken)
	require.NotEmpty(u.T(), response.Token.RefreshToken)

	require.Equal(u.T(), user.UID, response.UID)
	require.Equal(u.T(), user.FirstName, response.FirstName)
	require.Equal(u.T(), user.LastName, response.LastName)
	require.Equal(u.T(), user.Email, response.Email)
}

func (u *AuthIntegrationTestSuite) Test_LoginUser_Invalid_Username() {
	// Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
			"username": "%s",
			"password": "%s"
		}`, "random@test.com", "123456")

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *AuthIntegrationTestSuite) Test_LoginUser_Invalid_Password() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	// Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
			"username": "%s",
			"password": "%s"
		}`, user.Email, "12345")

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RefreshToken() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, token.AccessToken, token.RefreshToken)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response jwt.Token
	parseResponse(u.T(), w.Result(), &response)

	require.NotEmpty(u.T(), response.AccessToken)
	require.NotEmpty(u.T(), response.RefreshToken)
}

func (u *UserIntegrationTestSuite) Test_RefreshToken_Invalid_Access_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, ulid.Make().String(), token.RefreshToken)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RefreshToken_Invalid_Refresh_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, token.AccessToken, ulid.Make().String())

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_LogoutUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/logout"
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)
}

func (u *UserIntegrationTestSuite) Test_LogoutUser_Invalid_Access_Token() {
	// Arrange Request
	url := "/ui/auth/logout"
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", ulid.Make().String()))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func TestAuthIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(UserIntegrationTestSuite))
}
