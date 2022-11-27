//go:build integration
// +build integration

package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/internal/pkg/metrics"

	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	cm "github.com/frain-dev/convoy/datastore/mongo"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/server/testdb"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type UserIntegrationTestSuite struct {
	suite.Suite
	DB        cm.Client
	Router    http.Handler
	ConvoyApp *ApplicationHandler
	jwt       *jwt.Jwt
}

func (u *UserIntegrationTestSuite) SetupSuite() {
	u.DB = getDB()
	u.ConvoyApp = buildServer()
	u.Router = u.ConvoyApp.BuildRoutes()
}

func (u *UserIntegrationTestSuite) SetupTest() {
	testdb.PurgeDB(u.T(), u.DB)

	err := config.LoadConfig("./testdata/Auth_Config/jwt-convoy.json")
	require.NoError(u.T(), err)

	config, err := config.Get()
	require.NoError(u.T(), err)

	u.jwt = jwt.NewJwt(&config.Auth.Jwt, u.ConvoyApp.A.Cache)

	apiRepo := cm.NewApiKeyRepo(u.ConvoyApp.A.Store)
	userRepo := cm.NewUserRepo(u.ConvoyApp.A.Store)
	initRealmChain(u.T(), apiRepo, userRepo, u.ConvoyApp.A.Cache)
}

func (u *UserIntegrationTestSuite) TearDownTest() {
	testdb.PurgeDB(u.T(), u.DB)
	metrics.Reset()
}

func (u *UserIntegrationTestSuite) Test_LoginUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

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

	dbUser, err := cm.NewUserRepo(u.ConvoyApp.A.Store).FindUserByID(context.Background(), response.UID)
	require.NoError(u.T(), err)
	require.False(u.T(), dbUser.EmailVerified)
	require.NotEmpty(u.T(), dbUser.EmailVerificationToken)
	require.NotEmpty(u.T(), dbUser.EmailVerificationExpiresAt)
}

func (u *UserIntegrationTestSuite) Test_LoginUser_Invalid_Username() {
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

func (u *UserIntegrationTestSuite) Test_LoginUser_Invalid_Password() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

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

func (u *UserIntegrationTestSuite) Test_RegisterUser() {
	_, err := testdb.SeedConfiguration(u.ConvoyApp.A.Store)
	require.NoError(u.T(), err)

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"email": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.FirstName, r.LastName, r.Email, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusCreated, w.Code)

	var response models.LoginUserResponse
	parseResponse(u.T(), w.Result(), &response)

	require.NotEmpty(u.T(), response.UID)
	require.NotEmpty(u.T(), response.Token.AccessToken)
	require.NotEmpty(u.T(), response.Token.RefreshToken)

	require.Equal(u.T(), r.FirstName, response.FirstName)
	require.Equal(u.T(), r.LastName, response.LastName)
	require.Equal(u.T(), r.Email, response.Email)
}

func (u *UserIntegrationTestSuite) Test_RegisterUser_RegistrationNotAllowed() {
	config, err := testdb.SeedConfiguration(u.ConvoyApp.A.Store)
	require.NoError(u.T(), err)

	// disable registration
	config.IsSignupEnabled = false
	configRepo := cm.NewConfigRepo(u.ConvoyApp.A.Store)
	require.NoError(u.T(), configRepo.UpdateConfiguration(context.Background(), config))

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"email": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.FirstName, r.LastName, r.Email, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusForbidden, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RegisterUser_NoFirstName() {
	_, err := testdb.SeedConfiguration(u.ConvoyApp.A.Store)
	require.NoError(u.T(), err)

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"last_name": "%s",
		"email": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.LastName, r.Email, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RegisterUser_NoEmail() {
	_, err := testdb.SeedConfiguration(u.ConvoyApp.A.Store)
	require.NoError(u.T(), err)

	r := &models.RegisterUser{
		FirstName:        "test",
		LastName:         "test",
		Email:            "test@test.com",
		Password:         "123456",
		OrganisationName: "test",
	}
	// Arrange Request
	bodyStr := fmt.Sprintf(`{
		"first_name": "%s",
		"last_name": "%s",
		"password": "%s",
		"org_name": "%s"
	}`, r.FirstName, r.LastName, r.Password, r.OrganisationName)

	body := serialize(bodyStr)
	req := createRequest(http.MethodPost, "/ui/auth/register", "", body)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_RefreshToken() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

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
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, uuid.NewString(), token.RefreshToken)

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
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := "/ui/auth/token/refresh"
	bodyStr := fmt.Sprintf(`{
		"access_token": "%s",
		"refresh_token": "%s"
	}`, token.AccessToken, uuid.NewString())

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
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

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
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", uuid.NewString()))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusUnauthorized, w.Code)
}

func (u *UserIntegrationTestSuite) Test_GetUser() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/profile", user.UID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
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
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	user.EmailVerified = true

	userRepo := cm.NewUserRepo(u.ConvoyApp.A.Store)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	firstName := fmt.Sprintf("test%s", uuid.New().String())
	lastName := fmt.Sprintf("test%s", uuid.New().String())
	email := fmt.Sprintf("%s@test.com", uuid.New().String())

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/profile", user.UID)
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

	dbUser, err := userRepo.FindUserByID(context.Background(), user.UID)

	require.Equal(u.T(), dbUser.UID, response.UID)
	require.Equal(u.T(), firstName, dbUser.FirstName)
	require.Equal(u.T(), lastName, dbUser.LastName)
	require.Equal(u.T(), email, dbUser.Email)
}

func (u *UserIntegrationTestSuite) Test_UpdatePassword() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	user.EmailVerified = true

	userRepo := cm.NewUserRepo(u.ConvoyApp.A.Store)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	newPassword := "123456789"

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/password", user.UID)
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

	dbUser, err := userRepo.FindUserByID(context.Background(), user.UID)

	p := datastore.Password{Plaintext: newPassword, Hash: []byte(dbUser.Password)}
	isMatch, err := p.Matches()

	require.NoError(u.T(), err)

	require.Equal(u.T(), dbUser.UID, response.UID)
	require.True(u.T(), isMatch)
}

func (u *UserIntegrationTestSuite) Test_UpdatePassword_Invalid_Current_Password() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	user.EmailVerified = true

	userRepo := cm.NewUserRepo(u.ConvoyApp.A.Store)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/password", user.UID)
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
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	user.EmailVerified = true

	userRepo := cm.NewUserRepo(u.ConvoyApp.A.Store)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	token, err := u.jwt.GenerateToken(user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/%s/password", user.UID)
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

func (u *UserIntegrationTestSuite) Test_Forgot_Password_Valid_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	newPassword := "123456789"

	// Arrange Request
	url := "/ui/users/forgot-password"
	bodyStr := fmt.Sprintf(`{"email":"%s"}`, user.Email)

	req := httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	userRepo := cm.NewUserRepo(u.ConvoyApp.A.Store)
	dbUser, err := userRepo.FindUserByEmail(context.Background(), user.Email)
	require.NoError(u.T(), err)

	var response datastore.User
	parseResponse(u.T(), w.Result(), &response)
	// Reset password
	url = fmt.Sprintf("/ui/users/reset-password?token=%s", dbUser.ResetPasswordToken)
	bodyStr = fmt.Sprintf(`{
		"password": "%s",
		"password_confirmation": "%s"
	}`, newPassword, newPassword)

	req = httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w = httptest.NewRecorder()
	u.Router.ServeHTTP(w, req)
	require.Equal(u.T(), http.StatusOK, w.Code)
	parseResponse(u.T(), w.Result(), &response)

	dbUser, err = userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(u.T(), err)

	p := datastore.Password{Plaintext: newPassword, Hash: []byte(dbUser.Password)}
	isMatch, err := p.Matches()

	require.NoError(u.T(), err)
	require.Equal(u.T(), dbUser.UID, response.UID)
	require.True(u.T(), isMatch)
}

func (u *UserIntegrationTestSuite) Test_Forgot_Password_Invalid_Token() {
	password := "123456"
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", password)

	newPassword := "123456789"

	// Arrange Request
	url := "/ui/users/forgot-password"
	bodyStr := fmt.Sprintf(`{"email":"%s"}`, user.Email)

	req := httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	// Reset password
	url = fmt.Sprintf("/ui/users/reset-password?token=%s", "fake-token")
	bodyStr = fmt.Sprintf(`{
		"password": "%s",
		"password_confirmation": "%s"
	}`, newPassword, newPassword)

	req = httptest.NewRequest(http.MethodPost, url, serialize(bodyStr))
	req.Header.Add("Content-Type", "application/json")
	w = httptest.NewRecorder()
	u.Router.ServeHTTP(w, req)
	require.Equal(u.T(), http.StatusBadRequest, w.Code)
}

func (u *UserIntegrationTestSuite) Test_VerifyEmail() {
	user, _ := testdb.SeedUser(u.ConvoyApp.A.Store, "", testdb.DefaultUserPassword)

	user.EmailVerificationToken = uuid.NewString()
	user.EmailVerificationExpiresAt = primitive.NewDateTimeFromTime(time.Now().Add(time.Hour))

	userRepo := cm.NewUserRepo(u.ConvoyApp.A.Store)
	err := userRepo.UpdateUser(context.Background(), user)
	require.NoError(u.T(), err)

	// Arrange Request
	url := fmt.Sprintf("/ui/users/verify_email?token=%s", user.EmailVerificationToken)

	req := createRequest(http.MethodPost, url, "", nil)
	w := httptest.NewRecorder()

	// Act
	u.Router.ServeHTTP(w, req)

	// Assert
	require.Equal(u.T(), http.StatusOK, w.Code)

	dbUser, err := userRepo.FindUserByID(context.Background(), user.UID)
	require.NoError(u.T(), err)
	require.True(u.T(), dbUser.EmailVerified)
}

func TestUserIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(UserIntegrationTestSuite))
}
