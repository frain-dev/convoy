package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/auth/realm_chain"

	"github.com/frain-dev/convoy/config"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRequirePermission_Basic(t *testing.T) {

	tt := []struct {
		name        string
		statusCode  int
		credentials string
		cfgPath     string
	}{
		{
			name:       "credentials not provided",
			statusCode: http.StatusUnauthorized,
			cfgPath:    "./testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "invalid credentials",
			statusCode:  http.StatusUnauthorized,
			credentials: "Basic --",
			cfgPath:     "./testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "invalid basic credentials",
			statusCode:  http.StatusUnauthorized,
			credentials: "Basic ZGFuaWVs",
			cfgPath:     "./testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "authorization failed",
			statusCode:  http.StatusUnauthorized,
			credentials: "Basic YWRtaW46dGVzdA==",
			cfgPath:     "./testdata/Auth_Config/basic-convoy.json",
		},
		{
			name:        "valid credentials",
			statusCode:  http.StatusOK,
			credentials: "Basic dGVzdDp0ZXN0",
			cfgPath:     "./testdata/Auth_Config/basic-convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			defer realm_chain.Reset()

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fn := requirePermission()(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
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

			verifyMatch(t, *recorder)
		})
	}
}

func TestRequirePermission_Noop(t *testing.T) {
	tt := []struct {
		name        string
		statusCode  int
		credentials string
		cfgPath     string
	}{
		{
			name:       "credentials not provided",
			statusCode: http.StatusOK,
			cfgPath:    "./testdata/Auth_Config/none-convoy.json",
		},
		{
			name:        "invalid credentials",
			statusCode:  http.StatusOK,
			credentials: "--",
			cfgPath:     "./testdata/Auth_Config/none-convoy.json",
		},
		{
			name:        "authorization failed",
			statusCode:  http.StatusOK,
			credentials: "YWRtaW46dGVzdA==",
			cfgPath:     "./testdata/Auth_Config/none-convoy.json",
		},
		{
			name:        "valid credentials",
			statusCode:  http.StatusOK,
			credentials: "dGVzdDp0ZXN0",
			cfgPath:     "./testdata/Auth_Config/none-convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			defer realm_chain.Reset()

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fn := requirePermission()(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.WriteHeader(http.StatusOK)

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

			verifyMatch(t, *recorder)
		})
	}
}
