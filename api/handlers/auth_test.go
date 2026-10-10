package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/services"
)

func TestHandler_GoogleOAuthToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock dependencies
	mockLicenser := mocks.NewMockLicenser(ctrl)
	mockCache := mocks.NewMockCache(ctrl)

	// Create handler
	handler := &Handler{
		A: &types.APIOptions{
			DB:       nil,
			Cache:    mockCache,
			Licenser: mockLicenser,
			Cfg: config.Configuration{
				Auth: config.AuthConfiguration{
					GoogleOAuth: config.GoogleOAuthOptions{
						Enabled: true,
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		setupMocks     func()
		expectedStatus int
		expectedError  string
	}{
		{
			name: "should_return_401_when_google_oauth_disabled",
			requestBody: map[string]interface{}{
				"id_token": "valid_token",
			},
			setupMocks: func() {
				handler.A.Cfg.Auth.GoogleOAuth.Enabled = false
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "Google OAuth is not enabled",
		},
		{
			name: "should_return_400_when_id_token_missing",
			requestBody: map[string]interface{}{
				"id_token": "",
			},
			setupMocks: func() {
				handler.A.Cfg.Auth.GoogleOAuth.Enabled = true
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "missing ID token",
		},
		{
			name: "should_return_400_when_request_body_invalid",
			requestBody: map[string]interface{}{
				"invalid_field": "value",
			},
			setupMocks: func() {
				handler.A.Cfg.Auth.GoogleOAuth.Enabled = true
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "missing ID token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset handler state
			handler.A.Cfg.Auth.GoogleOAuth.Enabled = true

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/google/token", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			handler.GoogleOAuthToken(w, req)

			// Assert response
			require.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Contains(t, response["message"], tt.expectedError)
			}
		})
	}
}

func TestHandler_GoogleOAuthSetup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock dependencies
	mockLicenser := mocks.NewMockLicenser(ctrl)
	mockCache := mocks.NewMockCache(ctrl)

	// Create handler
	handler := &Handler{
		A: &types.APIOptions{
			DB:       nil,
			Cache:    mockCache,
			Licenser: mockLicenser,
			Cfg: config.Configuration{
				Auth: config.AuthConfiguration{
					GoogleOAuth: config.GoogleOAuthOptions{
						Enabled: true,
					},
				},
			},
		},
	}

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		setupMocks     func()
		expectedStatus int
		expectedError  string
	}{
		{
			name: "should_return_400_when_business_name_missing",
			requestBody: map[string]interface{}{
				"id_token":      "valid_token",
				"business_name": "",
			},
			setupMocks: func() {
				handler.A.Cfg.Auth.GoogleOAuth.Enabled = true
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Business name is required",
		},
		{
			name: "should_return_400_when_id_token_missing",
			requestBody: map[string]interface{}{
				"business_name": "Test Company",
				"id_token":      "",
			},
			setupMocks: func() {
				handler.A.Cfg.Auth.GoogleOAuth.Enabled = true
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "ID token is required",
		},
		{
			name: "should_return_400_when_request_body_invalid",
			requestBody: map[string]interface{}{
				"invalid_field": "value",
			},
			setupMocks: func() {
				handler.A.Cfg.Auth.GoogleOAuth.Enabled = true
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Business name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset handler state
			handler.A.Cfg.Auth.GoogleOAuth.Enabled = true

			// Setup mocks
			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			// Create request
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/auth/google/setup", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			handler.GoogleOAuthSetup(w, req)

			// Assert response
			require.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Contains(t, response["message"], tt.expectedError)
			}
		})
	}
}

func TestLoginUser(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		loginUserFn    func(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error)
		expectedStatus int
		expectedError  string
		expectedMsg    string
	}{
		{
			name: "non_ServiceError_returns_Authentication_failed_with_403",
			requestBody: models.LoginUser{
				Username: "user@example.com",
				Password: "password123",
			},
			loginUserFn: func(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
				return nil, nil, errors.New("unexpected database network error")
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "Authentication failed",
		},
		{
			name: "ServiceError_license_expired_returns_se_ErrMsg_with_403",
			requestBody: models.LoginUser{
				Username: "user@example.com",
				Password: "password123",
			},
			loginUserFn: func(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
				return nil, nil, &services.ServiceError{
					Code:   services.ErrCodeLicenseExpired,
					ErrMsg: "License expired. Only the first organization administrator can access the system",
				}
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "License expired. Only the first organization administrator can access the system",
		},
		{
			name: "ServiceError_internal_returns_500",
			requestBody: models.LoginUser{
				Username: "user@example.com",
				Password: "password123",
			},
			loginUserFn: func(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
				return nil, nil, &services.ServiceError{
					Code:   services.ErrCodeInternal,
					ErrMsg: "failed to evaluate license access",
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Service temporarily unavailable",
		},
		{
			name: "ServiceError_default_returns_invalid_credentials_with_403",
			requestBody: models.LoginUser{
				Username: "user@example.com",
				Password: "wrongpassword",
			},
			loginUserFn: func(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
				return nil, nil, &services.ServiceError{
					ErrMsg: "invalid username or password",
				}
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "Invalid credentials",
		},
		{
			name:           "invalid_json_body_returns_400",
			requestBody:    "not a json object",
			loginUserFn:    nil,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request format",
		},
		{
			name: "successful_login_returns_200",
			requestBody: models.LoginUser{
				Username: "user@example.com",
				Password: "password123",
			},
			loginUserFn: func(ctx context.Context, data *models.LoginUser) (*datastore.User, *jwt.Token, error) {
				return &datastore.User{
					UID:   "user-1",
					Email: "user@example.com",
				}, &jwt.Token{AccessToken: "access-token", RefreshToken: "refresh-token"}, nil
			},
			expectedStatus: http.StatusOK,
			expectedMsg:    "Login successful",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &Handler{
				A: &types.APIOptions{
					Logger: log.New("test", log.LevelInfo),
					Cfg: config.Configuration{
						Auth: config.AuthConfiguration{
							Jwt: config.JwtConfiguration{
								Secret:        "test-secret",
								RefreshSecret: "test-refresh-secret",
							},
						},
					},
				},
				loginUserFn: tt.loginUserFn,
			}

			var body []byte
			switch v := tt.requestBody.(type) {
			case string:
				body = []byte(v)
			default:
				var err error
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.LoginUser(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				require.Equal(t, "error", response["status"])
				require.NotEmpty(t, response["message"])
				require.Equal(t, tt.expectedError, response["message"])
			}

			if tt.expectedMsg != "" {
				require.Equal(t, "success", response["status"])
				require.Equal(t, tt.expectedMsg, response["message"])
			}
		})
	}
}
