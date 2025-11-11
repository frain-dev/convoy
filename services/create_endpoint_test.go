package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/pkg/log"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy"

	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideCreateEndpointService(ctrl *gomock.Controller, e models.CreateEndpoint, projectID string) *CreateEndpointService {
	return &CreateEndpointService{
		PortalLinkRepo: nil,
		EndpointRepo:   mocks.NewMockEndpointRepository(ctrl),
		ProjectRepo:    mocks.NewMockProjectRepository(ctrl),
		Licenser:       mocks.NewMockLicenser(ctrl),
		Logger:         log.NewLogger(os.Stdout),
		FeatureFlag:    fflag.NoopFflag(),
		E:              e,
		ProjectID:      projectID,
	}
}

// generateTestCertificate generates a valid self-signed certificate and private key for testing
func generateTestCertificate(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()

	// Generate private key
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Create certificate template
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Convoy Test"},
			CommonName:   "Test Client",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(24 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		KeyUsage:    x509.KeyUsageDigitalSignature,
	}

	// Create self-signed certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &privKey.PublicKey, privKey)
	require.NoError(t, err)

	// Encode certificate to PEM
	certPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}))

	// Encode private key to PEM
	keyPEM = string(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	}))

	return certPEM, keyPEM
}

func TestCreateEndpointService_Run(t *testing.T) {
	// Skip ping validation in tests since we use non-existent domains
	_ = os.Setenv("CONVOY_DISPATCHER_SKIP_PING_VALIDATION", "true")
	defer func() { _ = os.Unsetenv("CONVOY_DISPATCHER_SKIP_PING_VALIDATION") }()

	_ = config.LoadCaCert("", "")
	projectID := "1234567890"
	project := &datastore.Project{UID: projectID, Type: datastore.OutgoingProject, Config: &datastore.DefaultProjectConfig}

	// Generate valid test certificate for mTLS tests
	testCertPEM, testKeyPEM := generateTestCertificate(t)

	ctx := context.Background()
	type args struct {
		ctx context.Context
		e   models.CreateEndpoint
		g   *datastore.Project
	}
	tests := []struct {
		name         string
		args         args
		wantEndpoint *datastore.Endpoint
		dbFn         func(endpoint *CreateEndpointService)
		wantErr      bool
		wantErrMsg   string
	}{
		{
			name: "should_create_endpoint",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:            "endpoint",
					SupportEmail:    "endpoint@test.com",
					IsDisabled:      false,
					SlackWebhookURL: "https://google.com",
					HttpTimeout:     30,
					Secret:          "1234",
					URL:             "https://google.com",
					Description:     "test_endpoint",
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				p, _ := app.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(project, nil)

				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.HttpTimeout == 30
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := app.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantEndpoint: &datastore.Endpoint{
				Name:            "endpoint",
				SupportEmail:    "endpoint@test.com",
				SlackWebhookURL: "https://google.com",
				ProjectID:       project.UID,
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				AdvancedSignatures: true,
				HttpTimeout:        30,
				Url:                "https://google.com",
				Description:        "test_endpoint",
				RateLimit:          0,
				Status:             datastore.ActiveEndpointStatus,
				RateLimitDuration:  0,
			},
			wantErr: false,
		},
		{
			name: "should_fail_with_incomplete_mtls",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:        "mtls_endpoint_incomplete",
					Secret:      "1234",
					URL:         "https://google.com",
					Description: "endpoint with incomplete mTLS",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
						// missing client_key
					},
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				p, _ := app.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Times(1).Return(project, nil)

				licenser, _ := app.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
				licenser.EXPECT().MutualTLS().Times(1).Return(true)
			},
			wantErr:    true,
			wantErrMsg: "mtls_client_cert requires both client_cert and client_key",
		},
		{
			name: "should_default_http_timeout_endpoint_for_license_check_and_remove_slack_url_support_email",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:            "endpoint",
					SupportEmail:    "endpoint@test.com",
					IsDisabled:      false,
					SlackWebhookURL: "https://google.com",
					Secret:          "1234",
					URL:             "https://google.com",
					HttpTimeout:     3,
					Description:     "test_endpoint",
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				p, _ := app.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(project, nil)

				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.HttpTimeout == convoy.HTTP_TIMEOUT
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := app.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(false)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(false)
			},
			wantEndpoint: &datastore.Endpoint{
				Name:            "endpoint",
				SupportEmail:    "",
				SlackWebhookURL: "",
				ProjectID:       project.UID,
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				AdvancedSignatures: true,
				HttpTimeout:        convoy.HTTP_TIMEOUT,
				Url:                "https://google.com",
				Description:        "test_endpoint",
				RateLimit:          0,
				Status:             datastore.ActiveEndpointStatus,
				RateLimitDuration:  0,
			},
			wantErr: false,
		},
		{
			name: "should_create_endpoint_with_custom_authentication",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:              "endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: 60,
					URL:               "https://google.com",
					Description:       "test_endpoint",
					Authentication: &models.EndpointAuthentication{
						Type: datastore.APIKeyAuthentication,
						ApiKey: &models.ApiKey{
							HeaderName:  "x-api-key",
							HeaderValue: "x-api-key",
						},
					},
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				p, _ := app.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(project, nil)

				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				licenser, _ := app.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantEndpoint: &datastore.Endpoint{
				ProjectID: project.UID,
				Name:      "endpoint",
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				Url:                "https://google.com",
				AdvancedSignatures: true,
				Description:        "test_endpoint",
				RateLimit:          100,
				Status:             datastore.ActiveEndpointStatus,
				RateLimitDuration:  60,
				Authentication: &datastore.EndpointAuthentication{
					Type: datastore.APIKeyAuthentication,
					ApiKey: &datastore.ApiKey{
						HeaderName:  "x-api-key",
						HeaderValue: "x-api-key",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "should_create_endpoint_with_mtls",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:        "mtls_endpoint",
					Secret:      "1234",
					URL:         "https://secure.example.com",
					Description: "endpoint with mTLS",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: testCertPEM,
						ClientKey:  testKeyPEM,
					},
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				p, _ := app.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(project, nil)

				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Cond(func(x any) bool {
					endpoint := x.(*datastore.Endpoint)
					return endpoint.MtlsClientCert != nil &&
						endpoint.MtlsClientCert.ClientCert == testCertPEM &&
						endpoint.MtlsClientCert.ClientKey == testKeyPEM
				}), gomock.Any()).Times(1).Return(nil)

				licenser, _ := app.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
				licenser.EXPECT().MutualTLS().Times(1).Return(true)
			},
			wantEndpoint: &datastore.Endpoint{
				Name:      "mtls_endpoint",
				ProjectID: project.UID,
				Secrets: []datastore.Secret{
					{Value: "1234"},
				},
				AdvancedSignatures: true,
				Url:                "https://secure.example.com",
				Description:        "endpoint with mTLS",
				Status:             datastore.ActiveEndpointStatus,
				MtlsClientCert: &datastore.MtlsClientCert{
					ClientCert: testCertPEM,
					ClientKey:  testKeyPEM,
				},
			},
			wantErr: false,
		},
		{
			name: "should_fail_to_create_endpoint_with_mtls_when_license_denies",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:        "mtls_endpoint_denied",
					Secret:      "1234",
					URL:         "https://secure.example.com",
					Description: "endpoint with mTLS but license denies",
					MtlsClientCert: &models.MtlsClientCert{
						ClientCert: testCertPEM,
						ClientKey:  testKeyPEM,
					},
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				p, _ := app.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(project, nil)

				licenser, _ := app.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
				licenser.EXPECT().MutualTLS().Times(1).Return(false)
			},
			wantErr:    true,
			wantErrMsg: ErrMutualTLSFeatureUnavailable,
		},
		{
			name: "should_fail_to_create_endpoint",
			args: args{
				ctx: ctx,
				e: models.CreateEndpoint{
					Name:              "test_endpoint",
					Secret:            "1234",
					RateLimit:         100,
					RateLimitDuration: 60,
					URL:               "https://google.com",
					Description:       "test_endpoint",
				},
				g: project,
			},
			dbFn: func(app *CreateEndpointService) {
				p, _ := app.ProjectRepo.(*mocks.MockProjectRepository)
				p.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).
					Times(1).
					Return(project, nil)

				a, _ := app.EndpointRepo.(*mocks.MockEndpointRepository)
				a.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				licenser, _ := app.Licenser.(*mocks.MockLicenser)
				licenser.EXPECT().IpRules().Times(2).Return(true)
				licenser.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				licenser.EXPECT().CustomCertificateAuthority().Times(1).Return(true)
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while adding endpoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			as := provideCreateEndpointService(ctrl, tc.args.e, tc.args.g.UID)

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
			require.NotEmpty(t, endpoint.UID)
			require.NotEmpty(t, endpoint.CreatedAt)
			require.NotEmpty(t, endpoint.UpdatedAt)
			require.Empty(t, endpoint.DeletedAt)

			stripVariableFields(t, "endpoint", endpoint)
			require.Equal(t, tc.wantEndpoint, endpoint)
		})
	}
}
