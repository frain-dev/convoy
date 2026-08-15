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
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/api_keys"
	"github.com/frain-dev/convoy/internal/portal_links"
	"github.com/frain-dev/convoy/internal/users"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
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

	s.mockLicenser.EXPECT().IsMultiUserMode(gomock.Any()).Return(false, nil).AnyTimes()
	s.mockLicenser.EXPECT().AsynqMonitoring().Return(false).AnyTimes()
	s.mockLicenser.EXPECT().CheckOrgLimit(gomock.Any()).Return(true, nil).AnyTimes()
	s.mockLicenser.EXPECT().CheckUserLimit(gomock.Any()).Return(true, nil).AnyTimes()
	s.mockLicenser.EXPECT().CheckProjectLimit(gomock.Any()).Return(true, nil).AnyTimes()
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

	userRepo := users.New(log.New("convoy", log.LevelError), s.ConvoyApp.A.DB)
	err = userRepo.CreateUser(context.Background(), s.DefaultUser)
	require.NoError(s.T(), err)

	apiRepo := api_keys.New(s.ConvoyApp.A.Logger, s.ConvoyApp.A.DB)
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

	tl := newInfra(t)
	db := tl.Database
	cfg := tl.Config

	deps := newBroker(t, cfg, db, tl.Logger)

	ah, err := NewApplicationHandler(newAPIOptions(tl, deps, licenser))
	require.NoError(t, err)

	err = ah.RegisterPolicy()
	require.NoError(t, err)

	return ah
}

func TestOSSLoginIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(OSSLoginIntegrationTestSuite))
}
