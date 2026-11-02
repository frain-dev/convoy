package services

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func loadEndpointPingTestConfig(t *testing.T, skip bool) {
	t.Helper()
	t.Setenv("CONVOY_DISPATCHER_SKIP_PING_VALIDATION", strconv.FormatBool(skip))
	require.NoError(t, config.LoadConfig("", func(c *config.Configuration) error {
		if err := envconfig.Process("convoy", c); err != nil {
			return err
		}
		c.Dispatcher.PingMethods = []string{http.MethodPost}
		c.Dispatcher.InsecureSkipVerify = false
		c.Server.HTTP.HttpProxy = ""
		c.Server.HTTP.NoProxy = ""
		return nil
	}))
	t.Cleanup(func() {
		require.NoError(t, config.LoadConfig("", func(c *config.Configuration) error {
			c.Auth.Jwt.Secret = "test-access-secret"
			c.Auth.Jwt.RefreshSecret = "test-refresh-secret"
			return nil
		}))
	})
}

func validateEndpointPingTestURL(t *testing.T, operation, endpointURL string, auth *models.EndpointAuthentication) (string, error) {
	t.Helper()
	ctrl := gomock.NewController(t)
	projectConfig := datastore.DefaultProjectConfig
	projectConfig.SSL = &datastore.SSLConfiguration{EnforceSecureEndpoints: true}
	project := &datastore.Project{UID: "test-project", Config: &projectConfig}
	if operation == "create" {
		service := provideCreateEndpointService(ctrl, models.CreateEndpoint{URL: endpointURL, Authentication: auth}, project.UID)
		licenser := service.Licenser.(*mocks.MockLicenser)
		licenser.EXPECT().IpRules().AnyTimes().Return(false)
		licenser.EXPECT().CustomCertificateAuthority().AnyTimes().Return(true)
		return service.ValidateEndpoint(context.Background(), project, nil)
	}

	existing := &datastore.Endpoint{UID: "test-endpoint"}
	update := models.UpdateEndpoint{URL: endpointURL, Authentication: auth}
	if operation == "update_existing_auth" {
		existing.Authentication = &datastore.EndpointAuthentication{Type: auth.Type, OAuth2: auth.OAuth2.Transform()}
		update.Authentication = nil
	}
	service := provideUpdateEndpointService(ctrl, update, existing, project)
	licenser := service.Licenser.(*mocks.MockLicenser)
	licenser.EXPECT().IpRules().AnyTimes().Return(false)
	licenser.EXPECT().CustomCertificateAuthority().AnyTimes().Return(true)
	return service.ValidateEndpoint(context.Background(), project, nil, existing)
}

func TestEndpointPingValidation(t *testing.T) {
	for _, operation := range []string{"create", "update", "update_existing_auth"} {
		t.Run(operation, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				skip   bool
				status int
			}{
				{name: "disabled_success", skip: true, status: http.StatusOK},
				{name: "disabled_failure", skip: true, status: http.StatusBadRequest},
				{name: "enabled_success", status: http.StatusOK},
				{name: "enabled_failure", status: http.StatusBadRequest},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var pingRequests, tokenRequests atomic.Int32
					server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path == "/token" {
							tokenRequests.Add(1)
							w.Header().Set("Content-Type", "application/json")
							_, _ = w.Write([]byte(`{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`))
							return
						}
						pingRequests.Add(1)
						w.WriteHeader(tc.status)
					}))
					t.Cleanup(server.Close)
					cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
					require.NoError(t, config.LoadCaCert(string(cert), ""))
					t.Cleanup(func() { require.NoError(t, config.LoadCaCert("", "")) })
					loadEndpointPingTestConfig(t, tc.skip)
					auth := &models.EndpointAuthentication{
						Type: datastore.OAuth2Authentication,
						OAuth2: &models.OAuth2{
							URL: server.URL + "/token", ClientID: "test-client", ClientSecret: "test-secret",
							AuthenticationType: "shared_secret", GrantType: "client_credentials",
						},
					}
					endpointURL := server.URL + "/webhook"
					result, err := validateEndpointPingTestURL(t, operation, endpointURL, auth)
					if !tc.skip && tc.status != http.StatusOK {
						require.ErrorContains(t, err, "endpoint validation failed")
					} else {
						require.NoError(t, err)
						require.Equal(t, endpointURL, result)
					}
					var wantRequests int32
					if !tc.skip {
						wantRequests = 1
					}
					require.Equal(t, wantRequests, pingRequests.Load(), "endpoint requests")
					require.Equal(t, wantRequests, tokenRequests.Load(), "OAuth2 token requests")
				})
			}
		})
	}
}

func TestEndpointSkipPingPreservesURLValidation(t *testing.T) {
	loadEndpointPingTestConfig(t, true)
	for _, operation := range []string{"create", "update"} {
		t.Run(operation, func(t *testing.T) {
			for _, endpointURL := range []string{"", "://invalid", "ftp://example.com", "http://example.com"} {
				t.Run(endpointURL, func(t *testing.T) {
					_, err := validateEndpointPingTestURL(t, operation, endpointURL, nil)
					require.Error(t, err)
				})
			}
		})
	}
}
