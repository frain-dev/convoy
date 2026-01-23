package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/api/types"
	rcache "github.com/frain-dev/convoy/cache/redis"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/configuration"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	rlimiter "github.com/frain-dev/convoy/internal/pkg/limiter/redis"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	redisqueue "github.com/frain-dev/convoy/queue/redis"
)

type OSSLoginIntegrationTestSuite struct {
	suite.Suite
	Router       http.Handler
	ConvoyApp    *ApplicationHandler
	DefaultUser  *datastore.User
	mockCtrl     *gomock.Controller
	mockLicenser *mocks.MockLicenser
}

func (s *OSSLoginIntegrationTestSuite) SetupSuite() {
	s.mockCtrl = gomock.NewController(s.T())
	s.mockLicenser = mocks.NewMockLicenser(s.mockCtrl)

	s.mockLicenser.EXPECT().MultiPlayerMode().Return(false).AnyTimes()
	s.mockLicenser.EXPECT().AsynqMonitoring().Return(false).AnyTimes()
	s.mockLicenser.EXPECT().CreateOrg(gomock.Any()).Return(true, nil).AnyTimes()
	s.mockLicenser.EXPECT().CreateUser(gomock.Any()).Return(true, nil).AnyTimes()
	s.mockLicenser.EXPECT().CreateProject(gomock.Any()).Return(true, nil).AnyTimes()
	s.mockLicenser.EXPECT().ProjectEnabled(gomock.Any()).Return(true).AnyTimes()
	s.mockLicenser.EXPECT().AddEnabledProject(gomock.Any()).AnyTimes()
	s.mockLicenser.EXPECT().RemoveEnabledProject(gomock.Any()).AnyTimes()
	s.mockLicenser.EXPECT().FeatureListJSON(gomock.Any()).Return(nil, nil).AnyTimes()

	s.ConvoyApp = s.buildServerWithMockLicenser(s.T(), s.mockLicenser)
	s.Router = s.ConvoyApp.BuildControlPlaneRoutes()
}

func (s *OSSLoginIntegrationTestSuite) TearDownSuite() {
	if s.mockCtrl != nil {
		s.mockCtrl.Finish()
	}
}

func (s *OSSLoginIntegrationTestSuite) SetupTest() {
	err := config.LoadConfig("./testdata/Auth_Config/jwt-convoy.json")
	require.NoError(s.T(), err)

	p := datastore.Password{Plaintext: "default"}
	err = p.GenerateHash()
	require.NoError(s.T(), err)

	s.DefaultUser = &datastore.User{
		UID:           ulid.Make().String(),
		FirstName:     "default",
		LastName:      "default",
		Email:         "superuser@default.com",
		Password:      string(p.Hash),
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	userRepo := postgres.NewUserRepo(s.ConvoyApp.A.DB)
	err = userRepo.CreateUser(context.Background(), s.DefaultUser)
	require.NoError(s.T(), err)

	apiRepo := api_keys.New(nil, s.ConvoyApp.A.DB)
	portalLinkRepo := portal_links.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
	initRealmChain(s.T(), apiRepo, userRepo, portalLinkRepo, s.ConvoyApp.A.Cache)
}

func (s *OSSLoginIntegrationTestSuite) Test_OSSDefaultUserLogin_ShouldSucceed() {
	url := "/ui/auth/login"
	body := serialize(`{
		"username": "%s",
		"password": "%s"
	}`, s.DefaultUser.Email, "default")
	req := createRequest(http.MethodPost, url, "", body)
	w := httptest.NewRecorder()

	s.Router.ServeHTTP(w, req)

	require.Equal(s.T(), http.StatusOK, w.Code)

	var response models.LoginUserResponse
	parseResponse(s.T(), w.Result(), &response)

	require.NotEmpty(s.T(), response.UID)
	require.NotEmpty(s.T(), response.Token.AccessToken)
	require.NotEmpty(s.T(), response.Token.RefreshToken)

	require.Equal(s.T(), s.DefaultUser.UID, response.UID)
	require.Equal(s.T(), s.DefaultUser.FirstName, response.FirstName)
	require.Equal(s.T(), s.DefaultUser.LastName, response.LastName)
	require.Equal(s.T(), s.DefaultUser.Email, response.Email)
}

func (s *OSSLoginIntegrationTestSuite) buildServerWithMockLicenser(t *testing.T, licenser *mocks.MockLicenser) *ApplicationHandler {
	t.Helper()

	var qOpts queue.QueueOptions

	tl := newInfra(t)
	db := tl.Database

	qOpts, err := getQueueOptions(t, tl.Config)
	require.NoError(t, err)

	cfg := tl.Config

	newQueue := redisqueue.NewQueue(qOpts)

	noopCache := rcache.NewRedisCacheFromClient(tl.Redis)
	limiter := rlimiter.NewLimiterFromRedisClient(tl.Redis)

	ah, err := NewApplicationHandler(
		&types.APIOptions{
			DB:                         db,
			Queue:                      newQueue,
			Redis:                      tl.Redis,
			Logger:                     tl.Logger,
			Cache:                      noopCache,
			FFlag:                      fflag.NewFFlag([]string{string(fflag.Prometheus), string(fflag.FullTextSearch)}),
			FeatureFlagFetcher:         postgres.NewFeatureFlagFetcher(db),
			EarlyAdopterFeatureFetcher: postgres.NewEarlyAdopterFeatureFetcher(db),
			Rate:                       limiter,
			ConfigRepo:                 configuration.New(tl.Logger, db),
			Licenser:                   licenser,
			Cfg:                        cfg,
		})
	require.NoError(t, err)

	err = ah.RegisterPolicy()
	require.NoError(t, err)

	return ah
}

func TestOSSLoginIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OSSLoginIntegrationTestSuite))
}
