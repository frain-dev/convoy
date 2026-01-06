package services

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/constants"
	"github.com/frain-dev/convoy/pkg/log"
)

func provideUpdateEndpointService(ctrl *gomock.Controller, e models.UpdateEndpoint, Endpoint *datastore.Endpoint, Project *datastore.Project) *UpdateEndpointService {
	return &UpdateEndpointService{
		Cache:                      mocks.NewMockCache(ctrl),
		EndpointRepo:               mocks.NewMockEndpointRepository(ctrl),
		ProjectRepo:                mocks.NewMockProjectRepository(ctrl),
		Licenser:                   mocks.NewMockLicenser(ctrl),
		FeatureFlag:                fflag.NoopFflag(),
		FeatureFlagFetcher:         mocks.NewMockFeatureFlagFetcher(),
		EarlyAdopterFeatureFetcher: mocks.NewMockEarlyAdopterFeatureFetcherWithMTLSEnabled(),
		Logger:                     log.NewLogger(os.Stdout),
		E:                          e,
		Endpoint:                   Endpoint,
		Project:                    Project,
	}
}

func TestUpdateEndpointService_Run(t *testing.T) {
	_ = config.LoadCaCert("", "")
	ctx := context.Background()
	project := &datastore.Project{UID: "1234567890", Config: &datastore.DefaultProjectConfig}

	// Generate valid test certificate for mTLS tests
	testCertPEM, testKeyPEM := generateTestCertificate(t)
	type args struct {
		ctx      context.Context
		e        models.UpdateEndpoint
		endpoint *datastore.Endpoint
		project  *datastore.Project
	}
	tests := []struct {
		name         string
		args         args
		wantEndpoint *datastore.Endpoint
		dbFn         func(as *UpdateEndpointService)
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "should_update_endpoint",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Endpoint2"),
					Description:       "test_endpoint",
					URL:               "https://www.google.com/webhp",
					RateLimit:         10000,
					RateLimitDuration: 60,
					HttpTimeout:       20,
				},
				endpoint: &datastore.Endpoint{UID: "endpoint2"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Name:              "Endpoint2",
				Description:       "test_endpoint",
				Url:               "https://www.google.com/webhp",
				RateLimit:         10000,
				RateLimitDuration: 60,
				HttpTimeout:       20,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint2"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_update_endpoint",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Endpoint 1"),
					Description:       "test_endpoint",
					URL:               "https://www.google.com/webhp",
					RateLimit:         10000,
					RateLimitDuration: 60,
					HttpTimeout:       20,
				},
				endpoint: &datastore.Endpoint{UID: "endpoint1"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint1"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(errors.New("failed"))

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while updating endpoints",
		},
		{
			name: "should_default_endpoint_http_timeout_for_license_check_failed",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Endpoint2"),
					Description:       "test_endpoint",
					URL:               "https://www.google.com/webhp",
					RateLimit:         10000,
					RateLimitDuration: 60,
					HttpTimeout:       200,
				},
				endpoint: &datastore.Endpoint{UID: "endpoint2"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Name:              "Endpoint2",
				Description:       "test_endpoint",
				Url:               "https://www.google.com/webhp",
				RateLimit:         10000,
				RateLimitDuration: 60,
				HttpTimeout:       convoy.HTTP_TIMEOUT,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint2"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.HttpTimeout == convoy.HTTP_TIMEOUT
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(false)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(false)
			},
			wantErr: false,
		},
		{
			name: "should_error_for_endpoint_not_found",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("endpoint1"),
					Description:       "test_endpoint",
					URL:               "https://www.google.com/webhp",
					RateLimit:         10000,
					RateLimitDuration: 60,
					HttpTimeout:       20,
				},
				endpoint: &datastore.Endpoint{UID: "endpoint1"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(nil, datastore.ErrEndpointNotFound)

				// No licenser expectations - FindEndpointByID fails before ValidateEndpoint is called
			},
			wantErr:    true,
			wantErrMsg: "endpoint not found",
		},
		{
			name: "should_update_endpoint_with_form_urlencoded_content_type",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Form Endpoint"),
					Description:       "test_endpoint_with_form_data",
					URL:               "https://www.google.com/webhp",
					RateLimit:         10000,
					RateLimitDuration: 60,
					HttpTimeout:       20,
					ContentType:       stringPtr(constants.ContentTypeFormURLEncoded),
				},
				endpoint: &datastore.Endpoint{UID: "endpoint3"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Name:              "Form Endpoint",
				Description:       "test_endpoint_with_form_data",
				Url:               "https://www.google.com/webhp",
				RateLimit:         10000,
				RateLimitDuration: 60,
				HttpTimeout:       20,
				ContentType:       constants.ContentTypeFormURLEncoded,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint3"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.ContentType == constants.ContentTypeFormURLEncoded
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr: false,
		},
		{
			name: "should_update_endpoint_with_json_content_type",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("JSON Endpoint"),
					Description:       "test_endpoint_with_json",
					URL:               "https://www.google.com/webhp",
					RateLimit:         10000,
					RateLimitDuration: 60,
					HttpTimeout:       20,
					ContentType:       stringPtr(constants.ContentTypeJSON),
				},
				endpoint: &datastore.Endpoint{UID: "endpoint4"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Name:              "JSON Endpoint",
				Description:       "test_endpoint_with_json",
				Url:               "https://www.google.com/webhp",
				RateLimit:         10000,
				RateLimitDuration: 60,
				HttpTimeout:       20,
				ContentType:       constants.ContentTypeJSON,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint4"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.ContentType == constants.ContentTypeJSON
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr: false,
		},
		{
			name: "should_default_to_json_when_content_type_is_nil",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:              stringPtr("Default Endpoint"),
					Description:       "test_endpoint_with_default_content_type",
					URL:               "https://www.google.com/webhp",
					RateLimit:         10000,
					RateLimitDuration: 60,
					HttpTimeout:       20,
					ContentType:       nil,
				},
				endpoint: &datastore.Endpoint{UID: "endpoint5"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Name:              "Default Endpoint",
				Description:       "test_endpoint_with_default_content_type",
				Url:               "https://www.google.com/webhp",
				RateLimit:         10000,
				RateLimitDuration: 60,
				HttpTimeout:       20,
				ContentType:       "",
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").
					Times(1).Return(&datastore.Endpoint{UID: "endpoint5"}, nil)

				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.ContentType == ""
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr: false,
		},
		{
			name: "should_fail_with_incomplete_mtls",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:        stringPtr("Endpoint mTLS bad"),
					Description: "desc",
					URL:         "https://www.google.com/webhp",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
					},
				},
				endpoint: &datastore.Endpoint{UID: "endpoint-mtls-bad"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").Times(1).Return(&datastore.Endpoint{UID: "endpoint-mtls-bad"}, nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
				licenser.EXPECT().MutualTLS().Times(1).Return(true)
			},
			wantErr:    true,
			wantErrMsg: "mtls_client_cert requires both client_cert and client_key",
		},
		{
			name: "should_update_endpoint_with_mtls_field_omitted_uses_existing_cert",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:        stringPtr("Updated Name Only"),
					Description: "updating name without touching mTLS",
					URL:         "https://www.google.com/webhp",
					// MtlsClientCert is nil (field omitted) - should use existing cert from DB
				},
				endpoint: &datastore.Endpoint{UID: "endpoint-with-mtls"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Name:        "Updated Name Only",
				Description: "updating name without touching mTLS",
				Url:         "https://www.google.com/webhp",
				MtlsClientCert: &datastore.MtlsClientCert{
					ClientCert: "-----BEGIN CERTIFICATE-----\nexisting-cert\n-----END CERTIFICATE-----",
					ClientKey:  "-----BEGIN PRIVATE KEY-----\nexisting-key\n-----END PRIVATE KEY-----",
				},
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				// Return endpoint with existing mTLS cert
				existingEndpoint := &datastore.Endpoint{
					UID: "endpoint-with-mtls",
					MtlsClientCert: &datastore.MtlsClientCert{
						ClientCert: "-----BEGIN CERTIFICATE-----\nexisting-cert\n-----END CERTIFICATE-----",
						ClientKey:  "-----BEGIN PRIVATE KEY-----\nexisting-key\n-----END PRIVATE KEY-----",
					},
				}
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").Times(1).Return(existingEndpoint, nil)

				// Verify that mTLS cert is preserved (not removed)
				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.MtlsClientCert != nil &&
						endpoint.MtlsClientCert.ClientCert == "-----BEGIN CERTIFICATE-----\nexisting-cert\n-----END CERTIFICATE-----"
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_replace_mtls_cert_with_invalid_cert",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:        stringPtr("Endpoint with new cert"),
					Description: "replacing mTLS cert with invalid data",
					URL:         "https://www.google.com/webhp",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: "-----BEGIN CERTIFICATE-----\ninvalid-cert-data\n-----END CERTIFICATE-----",
						ClientKey:  "-----BEGIN PRIVATE KEY-----\ninvalid-key-data\n-----END PRIVATE KEY-----",
					},
				},
				endpoint: &datastore.Endpoint{UID: "endpoint-replace-mtls"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				// Return endpoint with existing mTLS cert
				existingEndpoint := &datastore.Endpoint{
					UID: "endpoint-replace-mtls",
					MtlsClientCert: &datastore.MtlsClientCert{
						ClientCert: "-----BEGIN CERTIFICATE-----\nold-cert\n-----END CERTIFICATE-----",
						ClientKey:  "-----BEGIN PRIVATE KEY-----\nold-key\n-----END PRIVATE KEY-----",
					},
				}
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").Times(1).Return(existingEndpoint, nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
				licenser.EXPECT().MutualTLS().Times(1).Return(true)
			},
			wantErr:    true,
			wantErrMsg: "invalid mTLS client certificate: failed to parse client certificate and key: tls: failed to find any PEM data in certificate input",
		},
		{
			name: "should_fail_to_update_endpoint_with_mtls_when_license_denies",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:        stringPtr("Endpoint with mTLS denied"),
					Description: "updating endpoint with mTLS but license denies",
					URL:         "https://www.google.com/webhp",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: testCertPEM,
						ClientKey:  testKeyPEM,
					},
				},
				endpoint: &datastore.Endpoint{UID: "endpoint-mtls-denied"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				existingEndpoint := &datastore.Endpoint{
					UID: "endpoint-mtls-denied",
				}
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").Times(1).Return(existingEndpoint, nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
				licenser.EXPECT().MutualTLS().Times(1).Return(false)
			},
			wantErr:    true,
			wantErrMsg: ErrMutualTLSFeatureUnavailable,
		},
		{
			name: "should_ignore_mtls_when_feature_flag_disabled",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:        stringPtr("Endpoint with mTLS disabled flag"),
					Description: "updating endpoint with mTLS but feature flag disabled",
					URL:         "https://www.google.com/webhp",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: testCertPEM,
						ClientKey:  testKeyPEM,
					},
				},
				endpoint: &datastore.Endpoint{UID: "endpoint-mtls-disabled-flag"},
				project:  project,
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				existingEndpoint := &datastore.Endpoint{
					UID: "endpoint-mtls-disabled-flag",
				}
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").Times(1).Return(existingEndpoint, nil)

				// Verify the mTLS cert is ignored (set to nil) when feature flag is disabled
				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.MtlsClientCert == nil
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
				licenser.EXPECT().MutualTLS().Times(1).Return(true)
			},
			wantEndpoint: &datastore.Endpoint{
				Name:        "Endpoint with mTLS disabled flag",
				Description: "updating endpoint with mTLS but feature flag disabled",
				Url:         "https://www.google.com/webhp",
				// mTLS should be nil when feature flag is disabled
				MtlsClientCert: nil,
			},
			wantErr: false,
		},
		{
			name: "should_remove_mtls_cert_with_empty_strings",
			args: args{
				ctx: ctx,
				e: models.UpdateEndpoint{
					Name:        stringPtr("Endpoint remove mTLS"),
					Description: "removing mTLS configuration",
					URL:         "https://www.google.com/webhp",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: "",
						ClientKey:  "",
					},
				},
				endpoint: &datastore.Endpoint{UID: "endpoint-remove-mtls"},
				project:  project,
			},
			wantEndpoint: &datastore.Endpoint{
				Name:        "Endpoint remove mTLS",
				Description: "removing mTLS configuration",
				Url:         "https://www.google.com/webhp",
			},
			dbFn: func(as *UpdateEndpointService) {
				a, _ := as.EndpointRepo.(*mocks.MockEndpointRepository)
				// Return endpoint with existing mTLS cert
				existingEndpoint := &datastore.Endpoint{
					UID: "endpoint-remove-mtls",
					MtlsClientCert: &datastore.MtlsClientCert{
						ClientCert: "-----BEGIN CERTIFICATE-----\nexisting-cert\n-----END CERTIFICATE-----",
						ClientKey:  "-----BEGIN PRIVATE KEY-----\nexisting-key\n-----END PRIVATE KEY-----",
					},
				}
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), "1234567890").Times(1).Return(existingEndpoint, nil)

				// Verify the mTLS cert is removed (set to nil)
				a.EXPECT().UpdateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.MtlsClientCert == nil
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := as.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().AnyTimes().Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideUpdateEndpointService(ctrl, tc.args.e, tc.args.endpoint, tc.args.project)

			// Override fetcher for feature flag disabled test
			if tc.name == "should_ignore_mtls_when_feature_flag_disabled" {
				as.FeatureFlagFetcher = mocks.NewMockFeatureFlagFetcher()
				as.EarlyAdopterFeatureFetcher = mocks.NewMockEarlyAdopterFeatureFetcherWithMTLSDisabled()
			}

			err := config.LoadConfig("")
			require.NoError(t, err)

			// Arrange Expectations
			if tc.dbFn != nil {
				tc.dbFn(as)
			}

			endpoint, err := as.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, endpoint.UpdatedAt)

			stripVariableFields(t, "endpoint", endpoint)
			require.Equal(t, tc.wantEndpoint, endpoint)
		})
	}
}
