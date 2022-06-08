//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UserIntegrationTestSuite struct {
	suite.Suite
	DB        datastore.DatabaseClient
	Router    http.Handler
	ConvoyApp *applicationHandler
	jwt       *jwt.Jwt
}

func (u *UserIntegrationTestSuite) SetupSuite() {
	u.DB = getDB()
	u.ConvoyApp = buildApplication()
	u.Router = buildRoutes(u.ConvoyApp)
}

func (u *UserIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(u.DB)

	err := config.LoadConfig("./testdata/Auth_Config/jwt-convoy.json")
	require.NoError(u.T(), err)

	config, err := config.Get()
	require.NoError(u.T(), err)

	u.jwt = jwt.NewJwt(&config.Auth.Jwt, u.ConvoyApp.cache)

	initRealmChain(u.T(), u.DB.APIRepo(), u.DB.UserRepo(), u.ConvoyApp.cache)
}

func (u *UserIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(u.DB)
}

func (u *UserIntegrationTestSuite) Test_LoginUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	//Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
		"username": "%s",
		"password": "%s"
	}`, user.Email, password)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
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

func (u *UserIntegrationTestSuite) Test_LoginUser_Invalid_Username() {
	//Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
			"username": "%s",
			"password": "%s"
		}`, "random@test.com", "123456")

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_LoginUser_Invalid_Password() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	//Arrange Request
	url := "/ui/auth/login"
	bodyStr := fmt.Sprintf(`{
			"username": "%s",
			"password": "%s"
		}`, user.Email, "12345")

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RefreshToken() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, token.AccessToken, token.RefreshToken)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response jwt.Token
	parseResponse(u.T(), w.Result(), &response)

	require.NotEmpty(u.T(), response.AccessToken)
	require.NotEmpty(u.T(), response.RefreshToken)

}

func (u *UserIntegrationTestSuite) Test_RefreshToken_Invalid_Access_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, uuid.NewString(), token.RefreshToken)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RefreshToken_Invalid_Refresh_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, token.AccessToken, uuid.NewString())

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, url, body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_LogoutUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

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

	//Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

}

func (u *UserIntegrationTestSuite) Test_LogoutUser_Invalid_Access_Token() {
	// Arrange Request
	url := "/ui/auth/logout"
	req := httptest.NewRequest(http.MethodPost, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", uuid.NewString()))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_GetUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/users/profile"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	//Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)

	require.Equal(u.T(), user.UID, response.UID)
	require.Equal(u.T(), user.FirstName, response.FirstName)
	require.Equal(u.T(), user.LastName, response.LastName)
	require.Equal(u.T(), user.Email, response.Email)
}

func (u *UserIntegrationTestSuite) Test_UpdateUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	firstName := fmt.Sprintf("test%s", uuid.New().String())
	lastName := fmt.Sprintf("test%s", uuid.New().String())
	email := fmt.Sprintf("%s@test.com", uuid.New().String())

	// Arrange Request
	url := "/ui/users/profile"
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"email": "%s"
	}`, firstName, lastName, email)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)

	dbUser, err := u.DB.UserRepo().FindUserByID(context.Background(), user.UID)

	require.Equal(u.T(), dbUser.UID, response.UID)
	require.Equal(u.T(), firstName, dbUser.FirstName)
	require.Equal(u.T(), lastName, dbUser.LastName)
	require.Equal(u.T(), email, dbUser.Email)

}

func (u *UserIntegrationTestSuite) Test_UpdatePassword() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	newPassword := "123456789"

	// Arrange Request
	url := "/ui/users/password"
	bodyStr := fmt.Sprintf(`{
		"current_password": "%s",
		"password": "%s",
		"password_confirmation": "%s"
	}`, password, newPassword, newPassword)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)

	dbUser, err := u.DB.UserRepo().FindUserByID(context.Background(), user.UID)

	p := datastore.Password{Plaintext: newPassword, Hash: []byte(dbUser.Password)}
	isMatch, err := p.Matches()

	require.NoError(u.T(), err)

	require.Equal(u.T(), dbUser.UID, response.UID)
	require.True(u.T(), isMatch)
}

func (u *UserIntegrationTestSuite) Test_UpdatePassword_Invalid_Current_Password() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/users/password"
	bodyStr := fmt.Sprintf(`{
		"current_password": "new-password",
		"password": "%s",
		"password_confirmation": "%s"
	}`, password, password)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_UpdatePassword_Invalid_Password_Confirmation() {
	password := "123456"
	user, _ := testdb.SeedUser(u.DB, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/users/password"
	bodyStr := fmt.Sprintf(`{
		"current_password": %s,
		"password": "%s",
		"password_confirmation": "new-password"
	}`, password, password)

	req := httptest.NewRequest(http.MethodPut, url, serialize(bodyStr))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func TestUserIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(UserIntegrationTestSuite))
}
