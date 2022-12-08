package fflag

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

type FF struct {
	cache       cache.Cache
	projectRepo datastore.ProjectRepository
}

func provideServices(ctrl *gomock.Controller) *FF {
	return &FF{
		cache:       mocks.NewMockCache(ctrl),
		projectRepo: mocks.NewMockProjectRepository(ctrl),
	}
}

func TestFeatureFlags_CLI(t *testing.T) {
	tt := []struct {
		name       string
		statusCode int
		IsEnabled  IsEnabledFunc
		mockFn     func(ff *FF)
		nFn        func() func()
		cfgPath    string
	}{
		{
			name:       "can_create_cli_api_key",
			statusCode: http.StatusOK,
			mockFn: func(ff *FF) {
				cache, _ := ff.cache.(*mocks.MockCache)
				cache.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				repo, _ := ff.projectRepo.(*mocks.MockProjectRepository)
				repo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{
					UID:            "123456",
					OrganisationID: "1234",
				}, nil)

				cache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("GET", "http://localhost:8080/api/v1/flags/can_create_cli_api_key",
					httpmock.NewStringResponder(200, `{"enabled": true}`))

				httpmock.RegisterResponder("POST", "http://localhost:8080/api/v1/evaluate",
					httpmock.NewStringResponder(200, `{"match": true }`))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			IsEnabled: Features[CanCreateCLIAPIKey],
			cfgPath:   "../../../server/testdata/Auth_Config/none-convoy.json",
		},

		{
			name:       "cannot_create_cli_api_key",
			statusCode: http.StatusForbidden,
			mockFn: func(ff *FF) {
				cache, _ := ff.cache.(*mocks.MockCache)
				cache.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				repo, _ := ff.projectRepo.(*mocks.MockProjectRepository)
				repo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{
					UID:            "123456",
					OrganisationID: "1234",
				}, nil)

				cache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("GET", "http://localhost:8080/api/v1/flags/can_create_cli_api_key",
					httpmock.NewStringResponder(200, `{"enabled": true}`))

				httpmock.RegisterResponder("POST", "http://localhost:8080/api/v1/evaluate",
					httpmock.NewStringResponder(200, `{"match": false }`))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			IsEnabled: Features[CanCreateCLIAPIKey],
			cfgPath:   "../../../server/testdata/Auth_Config/none-convoy.json",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ss := provideServices(ctrl)
			m := middleware.NewMiddleware(&middleware.CreateMiddleware{Cache: ss.cache, ProjectRepo: ss.projectRepo})

			fn := m.RequireProject()(CanAccessFeature(tc.IsEnabled)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`Hello`))

				require.NoError(t, err)
			})))

			if tc.mockFn != nil {
				tc.mockFn(ss)
			}

			if tc.nFn != nil {
				deferFn := tc.nFn()
				defer deferFn()
			}

			recorder := httptest.NewRecorder()

			bodyStr := strings.NewReader(`{"key_type": "cli"}`)
			request := httptest.NewRequest(http.MethodPost, "/?groupID=abc", bodyStr)

			fn.ServeHTTP(recorder, request)

			require.Equal(t, tc.statusCode, recorder.Code)
		})
	}
}
