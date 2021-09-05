package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/config"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRequireAuth_Misconfiguration(t *testing.T) {

	tt := []struct {
		name        string
		method      string
		statusCode  int
		credentials string
		cfgPath     string
	}{
		{
			name:       "credentials not provided",
			method:     http.MethodGet,
			statusCode: http.StatusForbidden,
			cfgPath:    "./testdata/TestRequireAuth_Misconfiguration/convoy.json",
		},
		{
			name:        "invalid credentials",
			method:      http.MethodGet,
			statusCode:  http.StatusForbidden,
			credentials: "--",
			cfgPath:     "./testdata/TestRequireAuth_Misconfiguration/convoy.json",
		},
		{
			name:        "authorization failed",
			method:      http.MethodGet,
			statusCode:  http.StatusForbidden,
			credentials: "YWRtaW46dGVzdA==",
			cfgPath:     "./testdata/TestRequireAuth_Misconfiguration/convoy.json",
		},
		{
			name:        "valid credentials",
			method:      http.MethodGet,
			statusCode:  http.StatusForbidden,
			credentials: "dGVzdDp0ZXN0",
			cfgPath:     "./testdata/TestRequireAuth_Misconfiguration/convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadFromFile(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fn := requireAuth()(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
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

func TestRequireAuth_None(t *testing.T) {

	tt := []struct {
		name        string
		method      string
		statusCode  int
		credentials string
		cfgPath     string
	}{
		{
			name:       "credentials not provided",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			cfgPath:    "./testdata/TestRequireAuth_None/convoy.json",
		},
		{
			name:        "invalid credentials",
			method:      http.MethodGet,
			statusCode:  http.StatusOK,
			credentials: "--",
			cfgPath:     "./testdata/TestRequireAuth_None/convoy.json",
		},
		{
			name:        "authorization failed",
			method:      http.MethodGet,
			statusCode:  http.StatusOK,
			credentials: "YWRtaW46dGVzdA==",
			cfgPath:     "./testdata/TestRequireAuth_None/convoy.json",
		},
		{
			name:        "valid credentials",
			method:      http.MethodGet,
			statusCode:  http.StatusOK,
			credentials: "dGVzdDp0ZXN0",
			cfgPath:     "./testdata/TestRequireAuth_None/convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadFromFile(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fn := requireAuth()(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
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

func TestRequireAuth_Basic(t *testing.T) {

	tt := []struct {
		name         string
		method       string
		statusCode   int
		responseBody string
		credentials  string
		cfgPath      string
	}{
		{
			name:         "credentials not provided",
			method:       http.MethodGet,
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"message":"invalid header structure"}`,
			cfgPath:      "./testdata/TestRequireAuth_Basic/convoy.json",
		},
		{
			name:         "invalid credentials",
			method:       http.MethodGet,
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"message":"invalid credentials"}`,
			credentials:  "--",
			cfgPath:      "./testdata/TestRequireAuth_Basic/convoy.json",
		},
		{
			name:         "authorization failed",
			method:       http.MethodGet,
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"message":"authorization failed"}`,
			credentials:  "YWRtaW46dGVzdA==",
			cfgPath:      "./testdata/TestRequireAuth_Basic/convoy.json",
		},
		{
			name:         "valid credentials",
			method:       http.MethodGet,
			statusCode:   http.StatusOK,
			credentials:  "dGVzdDp0ZXN0",
			responseBody: `Hello`,
			cfgPath:      "./testdata/TestRequireAuth_Basic/convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			err := config.LoadFromFile(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			fn := requireAuth()(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
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
