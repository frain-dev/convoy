package fflag

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type FF struct {
	featureFlag FeatureFlag
	cache       cache.Cache
	groupRepo   datastore.GroupRepository
}

func provideServices(ctrl *gomock.Controller) *FF {
	return &FF{
		featureFlag: mocks.NewMockFeatureFlag(ctrl),
		cache:       mocks.NewMockCache(ctrl),
		groupRepo:   mocks.NewMockGroupRepository(ctrl),
	}
}

func TestFeatureFlags_CLI(t *testing.T) {
	tt := []struct {
		name       string
		statusCode int
		IsEnabled  IsEnabledFunc
		mockFn     func(ff *FF)
	}{
		{
			name:       "can_create_cli_api_key",
			statusCode: http.StatusOK,
			mockFn: func(ff *FF) {
				cache, _ := ff.cache.(*mocks.MockCache)
				cache.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				repo, _ := ff.groupRepo.(*mocks.MockGroupRepository)
				repo.EXPECT().FetchGroupByID(gomock.Any(), gomock.Any()).Return(&datastore.Group{
					UID:            "123456",
					OrganisationID: "1234",
				}, nil)

				cache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				f, _ := ff.featureFlag.(*mocks.MockFeatureFlag)
				f.EXPECT().IsEnabled(gomock.Any(), gomock.Any()).Return(true, nil)
			},
			IsEnabled: Features[CanCreateCLIAPIKey],
		},

		{
			name:       "cannot_create_cli_api_key",
			statusCode: http.StatusForbidden,
			mockFn: func(ff *FF) {
				cache, _ := ff.cache.(*mocks.MockCache)
				cache.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				repo, _ := ff.groupRepo.(*mocks.MockGroupRepository)
				repo.EXPECT().FetchGroupByID(gomock.Any(), gomock.Any()).Return(&datastore.Group{
					UID:            "123456",
					OrganisationID: "1234",
				}, nil)

				cache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				f, _ := ff.featureFlag.(*mocks.MockFeatureFlag)
				f.EXPECT().IsEnabled(gomock.Any(), gomock.Any()).Return(false, nil)
			},
			IsEnabled: Features[CanCreateCLIAPIKey],
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ss := provideServices(ctrl)

			m := middleware.NewMiddleware(&middleware.CreateMiddleware{Cache: ss.cache, GroupRepo: ss.groupRepo})

			fn := m.RequireGroup()(CanAccessFeature(ss.featureFlag, tc.IsEnabled)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, err := w.Write([]byte(`Hello`))

				require.NoError(t, err)
			})))

			if tc.mockFn != nil {
				tc.mockFn(ss)
			}

			recorder := httptest.NewRecorder()

			bodyStr := strings.NewReader(`{"key_type": "cli"}`)
			request := httptest.NewRequest(http.MethodPost, "/?groupID=abc", bodyStr)

			fn.ServeHTTP(recorder, request)

			require.Equal(t, tc.statusCode, recorder.Code)
		})
	}
}
