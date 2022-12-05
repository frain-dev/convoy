//go:build integration
// +build integration

package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRequirePermission_Basic(t *testing.T) {
	m := &Middleware{
		logger: log.NewLogger(os.Stdout),
	}

	tt := []struct {
		name        string
		statusCode  int
		credentials string
		cfgPath     string
	}{
		{
			name:       "credentials not provided",
			statusCode: http.StatusUnauthorized,
			cfgPath:    "../../../server/testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "invalid credentials",
			statusCode:  http.StatusUnauthorized,
			credentials: "Basic --",
			cfgPath:     "../../../server/testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "invalid basic credentials",
			statusCode:  http.StatusUnauthorized,
			credentials: "Basic ZGFuaWVs",
			cfgPath:     "../../../server/testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "authorization failed",
			statusCode:  http.StatusUnauthorized,
			credentials: "Basic YWRtaW46dGVzdA==",
			cfgPath:     "../../../server/testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "valid credentials",
			statusCode:  http.StatusOK,
			credentials: "Basic dGVzdDp0ZXN0",
			cfgPath:     "../../../server/testdata/Auth_Config/basic-convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, nil, nil, nil)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fn := m.RequireAuth()(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.WriteHeader(http.StatusOK)

				_, err := rw.Write([]byte(`Hello`))
				require.NoError(t, err)
			}))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			if tc.credentials != "" {
				request.Header.Add("Authorization", tc.credentials)
			}

			fn.ServeHTTP(recorder, request)

			if recorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, recorder.Code)
			}
		})
	}
}

func TestRequirePermission_Noop(t *testing.T) {
	m := &Middleware{
		logger: log.NewLogger(os.Stdout),
	}

	tt := []struct {
		name        string
		statusCode  int
		credentials string
		cfgPath     string
	}{
		{
			name:       "credentials not provided",
			statusCode: http.StatusUnauthorized,
			cfgPath:    "./testdata/Auth_Config/none-convoy.json",
		},
		{
			name:        "invalid credentials",
			statusCode:  http.StatusUnauthorized,
			credentials: "--",
			cfgPath:     "./testdata/Auth_Config/none-convoy.json",
		},
		{
			name:        "authorization failed",
			statusCode:  http.StatusUnauthorized,
			credentials: "YWRtaW46dGVzdA==",
			cfgPath:     "./testdata/Auth_Config/none-convoy.json",
		},
		{
			name:        "valid credentials",
			statusCode:  http.StatusUnauthorized,
			credentials: "dGVzdDp0ZXN0",
			cfgPath:     "./testdata/Auth_Config/none-convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}
			initRealmChain(t, nil, nil, nil)

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fn := m.RequireAuth()(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.WriteHeader(http.StatusUnauthorized)

				_, err := rw.Write([]byte(`Hello`))
				require.NoError(t, err)
			}))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			if tc.credentials != "" {
				request.Header.Add("Authorization", fmt.Sprintf("Basic %s", tc.credentials))
			}

			fn.ServeHTTP(recorder, request)

			if recorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, recorder.Code)
			}
		})
	}
}

func TestRateLimitByProject(t *testing.T) {
	m := &Middleware{
		logger: log.NewLogger(os.Stdout),
	}

	type test struct {
		name          string
		requestsLimit int
		windowLength  time.Duration
		projectIDs    []string
		respCodes     []int
	}
	tests := []test{
		{
			name:          "no-block",
			requestsLimit: 3,
			windowLength:  2 * time.Second,
			projectIDs:    []string{"a", "a"},
			respCodes:     []int{200, 200},
		},
		{
			name:          "block-same-project",
			requestsLimit: 2,
			windowLength:  5 * time.Second,
			projectIDs:    []string{"b", "b", "b"},
			respCodes:     []int{200, 200, 429},
		},
		{
			name:          "no-block-different-project",
			requestsLimit: 1,
			windowLength:  1 * time.Second,
			projectIDs:    []string{"c", "d"},
			respCodes:     []int{200, 200},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			})
			router := m.RateLimitByProjectWithParams(tt.requestsLimit, tt.windowLength)(h)

			for i, code := range tt.respCodes {
				req := httptest.NewRequest("POST", "/", nil)
				req = req.Clone(context.WithValue(req.Context(), projectCtx, &datastore.Project{UID: tt.projectIDs[i]}))
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)
				if respCode := recorder.Result().StatusCode; respCode != code {
					t.Errorf("resp.StatusCode(%v) = %v, want %v", i, respCode, code)
				}
			}
		})
	}
}

func initRealmChain(t *testing.T, apiKeyRepo datastore.APIKeyRepository, userRepo datastore.UserRepository, cache cache.Cache) {
	cfg, err := config.Get()
	if err != nil {
		t.Errorf("failed to get config: %v", err)
	}

	err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, cache)
	if err != nil {
		t.Errorf("failed to initialize realm chain : %v", err)
	}
}
