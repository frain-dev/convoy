package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
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
