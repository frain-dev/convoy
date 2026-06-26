package task

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/net"
	cb "github.com/frain-dev/convoy/pkg/circuit_breaker"
	"github.com/frain-dev/convoy/pkg/clock"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

func TestResolveEventDeliveryTargetURL(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      *datastore.Endpoint
		eventDelivery *datastore.EventDelivery
		wantURL       string
		wantErr       error
	}{
		{
			name:     "uses concrete target URL when present",
			endpoint: &datastore.Endpoint{Url: "https://example.com/orders/{reference}/callback"},
			eventDelivery: &datastore.EventDelivery{
				TargetURL: "https://example.com/orders/123/callback",
			},
			wantURL: "https://example.com/orders/123/callback",
		},
		{
			name:     "appends query params to concrete target URL",
			endpoint: &datastore.Endpoint{Url: "https://example.com/orders/{reference}/callback"},
			eventDelivery: &datastore.EventDelivery{
				TargetURL:      "https://example.com/orders/123/callback?reference=ORD-123",
				URLQueryParams: "source=mobile",
			},
			wantURL: "https://example.com/orders/123/callback?reference=ORD-123&source=mobile",
		},
		{
			name:     "falls back to regular endpoint URL",
			endpoint: &datastore.Endpoint{Url: "https://example.com/callback"},
			eventDelivery: &datastore.EventDelivery{
				URLQueryParams: "source=mobile",
			},
			wantURL: "https://example.com/callback?source=mobile",
		},
		{
			name:          "fails closed for templated endpoint without target URL",
			endpoint:      &datastore.Endpoint{Url: "https://example.com/orders/{reference}/callback"},
			eventDelivery: &datastore.EventDelivery{},
			wantErr:       errEndpointURLTemplateTargetMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveEventDeliveryTargetURL(tt.endpoint, tt.eventDelivery)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantURL, got)
		})
	}
}

func TestProcessEventDelivery(t *testing.T) {
	badRequestServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
	}))
	defer badRequestServer.Close()
	tt := []struct {
		name          string
		cfgPath       string
		expectedError error
		msg           *datastore.EventDelivery
		dbFn          func(*mocks.MockEndpointRepository, *mocks.MockProjectRepository, *mocks.MockEventDeliveryRepository, *mocks.MockQueuer, *mocks.MockRateLimiter, *mocks.MockDeliveryAttemptsRepository, *mocks.MockLicenser, *mocks.MockBackend)
		nFn           func() func()
	}{
		{
			name:          "Event already sent.",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						EndpointID:     "endpoint-id-1",
						SubscriptionID: "sub-id-1",
						ProjectID:      "project-id-1",
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status:       datastore.SuccessEventStatus,
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				endpoint := &datastore.Endpoint{UID: "endpoint-id-1"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).Times(1).Return(endpoint, nil)

				project := &datastore.Project{UID: "project-id-1"}
				o.EXPECT().FetchProjectByID(gomock.Any(), "project-id-1").Times(1).Return(project, nil)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(2).Return(true)
			},
		},
		{
			name:          "Endpoint is inactive",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: 60,
						Status:            datastore.InactiveEndpointStatus,
					}, nil)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				o.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{Config: &datastore.DefaultProjectConfig}, nil)
				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						SubscriptionID: "sub-id-1",
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				m.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.DiscardedEventStatus).Times(1).Return(nil)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(2).Return(true)
			},
		},
		{
			name:          "Endpoint does not respond with 2xx",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID:         "123",
						RateLimit:         10,
						RateLimitDuration: 60,
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						Status: datastore.ActiveEndpointStatus,
					}, nil)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						SubscriptionID: "sub-id-1",
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status:       datastore.ScheduledEventStatus,
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: "X-Convoy-Signature",
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit: &datastore.DefaultRateLimitConfig,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(400, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Max retries reached - disabled endpoint - failed",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: 60,
						ProjectID:         "123",
						Status:            datastore.ActiveEndpointStatus,
					}, nil)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       2,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status:       datastore.ScheduledEventStatus,
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().
					UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(200, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Manual retry - disable endpoint - failed",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: 60,
						Status:            datastore.ActiveEndpointStatus,
					}, nil)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				a.EXPECT().
					UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.InactiveEndpointStatus).
					Return(nil).Times(1)

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status:       datastore.ScheduledEventStatus,
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.StrategyProvider("default"),
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(400, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Manual retry - disable endpoint - failed",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: 60,
						Status:            datastore.ActiveEndpointStatus,
					}, nil)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status: datastore.ScheduledEventStatus,
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().
					UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(400, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Manual retry - disable endpoint - success",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Url:       "https://google.com?source=giphy",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: 60,
						Status:            datastore.ActiveEndpointStatus,
					}, nil)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						SubscriptionID: "sub-id-1",
						Status:         datastore.ScheduledEventStatus,
						URLQueryParams: "name=ref&category=food",
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				a.EXPECT().
					UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.InactiveEndpointStatus).
					Return(nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com?category=food&name=ref&source=giphy",
					httpmock.NewStringResponder(200, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Manual retry - disable endpoint - success",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: 60,
						Status:            datastore.ActiveEndpointStatus,
					}, nil).Times(1)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Status: datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().
					UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(200, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Manual retry - disable endpoint - success - advanced endpoint mgmt false",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: 60,
						Status:            datastore.ActiveEndpointStatus,
					}, nil).Times(1)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						SubscriptionID: "sub-id-1",
						Status:         datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(false)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(200, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Manual retry - send disable endpoint notification",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID:    "123",
						SupportEmail: "test@gmail.com",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						Status:            datastore.ActiveEndpointStatus,
						RateLimitDuration: 60,
					}, nil).Times(1)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						SubscriptionID: "sub-id-1",
						Status:         datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				q.EXPECT().
					Write(gomock.Any(), convoy.NotificationProcessor, convoy.DefaultQueue, gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(200, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "Manual retry - send endpoint enabled notification",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID:    "123",
						SupportEmail: "test@gmail.com",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						Url:               "https://google.com",
						RateLimitDuration: 60,
						Status:            datastore.ActiveEndpointStatus,
					}, nil).Times(1)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						SubscriptionID: "sub-id-1",
						Status:         datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						DeliveryMode: datastore.AtLeastOnceDeliveryMode,
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: "X-Convoy-Signature",
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit:       &datastore.DefaultRateLimitConfig,
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				q.EXPECT().
					Write(gomock.Any(), convoy.NotificationProcessor, convoy.DefaultQueue, gomock.Any()).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(true)
				l.EXPECT().CircuitBreaking().Times(1).Return(false)
				l.EXPECT().AdvancedEndpointMgmt().Times(1).Return(true)
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder("POST", "https://google.com",
					httpmock.NewStringResponder(200, ``))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
		},
		{
			name:          "At-most-once delivery - non-2xx response - should not retry",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository, l *mocks.MockLicenser, mt *mocks.MockBackend) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						Url:       badRequestServer.URL,
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: 60,
						Status:            datastore.ActiveEndpointStatus,
					}, nil)

				r.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

				m.EXPECT().
					FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status:       datastore.ScheduledEventStatus,
						DeliveryMode: datastore.AtMostOnceDeliveryMode,
					}, nil).Times(1)

				o.EXPECT().
					FetchProjectByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Project{
						LogoURL: "",
						Config: &datastore.ProjectConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: "X-Convoy-Signature",
								Versions: []datastore.SignatureVersion{
									{
										UID:      "abc",
										Hash:     "SHA256",
										Encoding: datastore.HexEncoding,
									},
								},
							},
							SSL: &datastore.DefaultSSLConfig,
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							RateLimit: &datastore.DefaultRateLimitConfig,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				d.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Times(1)

				m.EXPECT().
					UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(&datastore.EventDelivery{})).
					DoAndReturn(func(ctx context.Context, projectID string, delivery *datastore.EventDelivery) error {
						assert.Equal(t, "Endpoint returned status code 400", delivery.Description)
						return nil
					}).
					Return(nil).Times(1)

				l.EXPECT().UseForwardProxy().Times(1).Return(true)
				l.EXPECT().IpRules().Times(3).Return(false)
			},
			nFn: nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			projectRepo := mocks.NewMockProjectRepository(ctrl)
			endpointRepo := mocks.NewMockEndpointRepository(ctrl)
			msgRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			userRepo := mocks.NewMockUserRepository(ctrl)
			cache := mocks.NewMockCache(ctrl)
			portalLinkRepo := mocks.NewMockPortalLinkRepository(ctrl)
			q := mocks.NewMockQueuer(ctrl)
			rateLimiter := mocks.NewMockRateLimiter(ctrl)
			attemptsRepo := mocks.NewMockDeliveryAttemptsRepository(ctrl)
			licenser := mocks.NewMockLicenser(ctrl)
			licenser.EXPECT().ProjectEnabled(gomock.Any()).Return(true).AnyTimes()
			mt := mocks.NewMockBackend(ctrl)

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			cfg, err := config.Get()
			if err != nil {
				t.Errorf("failed to get config: %v", err)
			}

			logger := mocks.NewMockLogger(ctrl)

			err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, cache, logger)
			if err != nil {
				t.Errorf("failed to initialize realm chain : %v", err)
			}

			if tc.nFn != nil {
				deferFn := tc.nFn()
				defer deferFn()
			}

			if tc.dbFn != nil {
				tc.dbFn(endpointRepo, projectRepo, msgRepo, q, rateLimiter, attemptsRepo, licenser, mt)
			}

			featureFlag := fflag.NewFFlag(cfg.EnableFeatureFlag)

			dispatcher, err := net.NewDispatcher(
				licenser,
				fflag.NewFFlag([]string{string(fflag.IpRules)}),
				net.LoggerOption(log.New("convoy", log.LevelInfo)),
				net.BlockListOption([]string{"10.0.0.0/8"}),
				net.ProxyOption("nil"),
			)
			require.NoError(t, err)

			mockStore := cb.NewTestStore()
			mockClock := clock.NewSimulatedClock(time.Now())
			breakerConfig := &cb.CircuitBreakerConfig{
				SampleRate:                  1,
				BreakerTimeout:              30,
				FailureThreshold:            50,
				SuccessThreshold:            2,
				ObservabilityWindow:         5,
				MinimumRequestCount:         10,
				ConsecutiveFailureThreshold: 3,
			}

			manager, err := cb.NewCircuitBreakerManager(
				cb.StoreOption(mockStore),
				cb.ClockOption(mockClock),
				cb.ConfigProviderOption(func(projectID string) *cb.CircuitBreakerConfig {
					return breakerConfig
				}),
				cb.LoggerOption(log.New("convoy", log.LevelInfo)),
			)
			require.NoError(t, err)

			// Create a nil fetcher for tests (will fall back to system-wide config)
			var fetcher fflag.FeatureFlagFetcher = nil

			deps := EventDeliveryProcessorDeps{
				EndpointRepo:          endpointRepo,
				EventDeliveryRepo:     msgRepo,
				Licenser:              licenser,
				ProjectRepo:           projectRepo,
				Queue:                 q,
				RateLimiter:           rateLimiter,
				Dispatcher:            dispatcher,
				AttemptsRepo:          attemptsRepo,
				CircuitBreakerManager: manager,
				FeatureFlag:           featureFlag,
				FeatureFlagFetcher:    fetcher,
				Logger:                log.New("convoy", log.LevelInfo),
			}
			processor := ProcessEventDelivery(deps)

			payload := EventDelivery{
				EventDeliveryID: tc.msg.UID,
				ProjectID:       tc.msg.ProjectID,
			}

			data, err := json.Marshal(payload)
			if err != nil {
				t.Errorf("failed to marshal payload: %v", err)
			}

			job := queue.Job{
				Payload: data,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			err = processor(context.Background(), task)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestProcessEventDelivery_SyncsAsynqMaxRetryToRetryLimit(t *testing.T) {
	cases := []struct {
		name         string
		retryLimit   uint64
		wantMaxRetry int
	}{
		{
			// Above asynq's default of 25: the configured value must win so the
			// delivery is not silently capped at 25.
			name:         "retry limit above asynq default is honored",
			retryLimit:   30,
			wantMaxRetry: 30,
		},
		{
			// At or below the default: the budget is floored at 25 to preserve
			// headroom for transient pre-dispatch errors. The configured count is
			// still enforced by the NumTrials check.
			name:         "retry limit below asynq default is floored",
			retryLimit:   5,
			wantMaxRetry: defaultAsynqMaxRetries,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			endpointRepo := mocks.NewMockEndpointRepository(ctrl)
			projectRepo := mocks.NewMockProjectRepository(ctrl)
			msgRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			q := mocks.NewMockQueuer(ctrl)
			rateLimiter := mocks.NewMockRateLimiter(ctrl)
			attemptsRepo := mocks.NewMockDeliveryAttemptsRepository(ctrl)
			licenser := mocks.NewMockLicenser(ctrl)
			licenser.EXPECT().ProjectEnabled(gomock.Any()).Return(true).AnyTimes()
			licenser.EXPECT().UseForwardProxy().Return(true).AnyTimes()
			licenser.EXPECT().IpRules().Return(true).AnyTimes()
			licenser.EXPECT().AdvancedEndpointMgmt().Return(false).AnyTimes()
			licenser.EXPECT().CircuitBreaking().Return(false).AnyTimes()

			require.NoError(t, config.LoadConfig("./testdata/Config/basic-convoy.json"))

			httpmock.Activate()
			defer httpmock.DeactivateAndReset()
			httpmock.RegisterResponder("POST", "https://google.com", httpmock.NewStringResponder(400, ``))

			endpointRepo.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&datastore.Endpoint{
					UID:               "endpoint-id-1",
					ProjectID:         "123",
					Url:               "https://google.com",
					RateLimit:         10,
					RateLimitDuration: 60,
					Secrets:           []datastore.Secret{{Value: "secret"}},
					Status:            datastore.ActiveEndpointStatus,
				}, nil)

			rateLimiter.EXPECT().AllowWithDuration(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			msgRepo.EXPECT().FindEventDeliveryByIDSlim(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&datastore.EventDelivery{
					UID:            "evt-del-1",
					EndpointID:     "endpoint-id-1",
					SubscriptionID: "sub-id-1",
					ProjectID:      "123",
					Metadata: &datastore.Metadata{
						Data:            []byte(`{"event":"x"}`),
						Raw:             `{"event":"x"}`,
						NumTrials:       0,
						RetryLimit:      tc.retryLimit,
						IntervalSeconds: 20,
					},
					Status:       datastore.ScheduledEventStatus,
					DeliveryMode: datastore.AtLeastOnceDeliveryMode,
				}, nil)

			projectRepo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).
				Return(&datastore.Project{
					UID: "123",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header:   "X-Convoy-Signature",
							Versions: []datastore.SignatureVersion{{UID: "abc", Hash: "SHA256", Encoding: datastore.HexEncoding}},
						},
						SSL:       &datastore.DefaultSSLConfig,
						Strategy:  &datastore.StrategyConfiguration{Type: datastore.LinearStrategyProvider, Duration: 60, RetryCount: tc.retryLimit},
						RateLimit: &datastore.DefaultRateLimitConfig,
					},
				}, nil)

			msgRepo.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
			attemptsRepo.EXPECT().CreateDeliveryAttempt(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			msgRepo.EXPECT().UpdateEventDeliveryMetadata(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

			var captured *queue.Job
			q.EXPECT().Write(gomock.Any(), convoy.RetryEventProcessor, convoy.RetryEventQueue, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ convoy.TaskName, _ convoy.QueueName, job *queue.Job) error {
					captured = job
					return nil
				}).Times(1)

			dispatcher, err := net.NewDispatcher(
				licenser,
				fflag.NewFFlag([]string{string(fflag.IpRules)}),
				net.LoggerOption(log.New("convoy", log.LevelInfo)),
				net.BlockListOption([]string{"10.0.0.0/8"}),
				net.ProxyOption("nil"),
			)
			require.NoError(t, err)

			deps := EventDeliveryProcessorDeps{
				EndpointRepo:      endpointRepo,
				EventDeliveryRepo: msgRepo,
				Licenser:          licenser,
				ProjectRepo:       projectRepo,
				Queue:             q,
				RateLimiter:       rateLimiter,
				Dispatcher:        dispatcher,
				AttemptsRepo:      attemptsRepo,
				FeatureFlag:       fflag.NewFFlag([]string{}),
				Logger:            log.New("convoy", log.LevelInfo),
			}

			payload, err := json.Marshal(EventDelivery{EventDeliveryID: "evt-del-1", ProjectID: "123"})
			require.NoError(t, err)
			task := asynq.NewTask(string(convoy.EventProcessor), payload, asynq.Queue(string(convoy.EventQueue)))

			require.NoError(t, ProcessEventDelivery(deps)(context.Background(), task))

			require.NotNil(t, captured, "expected a retry job to be enqueued")
			require.NotNil(t, captured.MaxRetry, "retry job should carry a synced asynq max retry")
			assert.Equal(t, tc.wantMaxRetry, *captured.MaxRetry)
		})
	}
}

func TestProcessEventDeliveryConfig(t *testing.T) {
	tt := []struct {
		name                string
		subscription        *datastore.Subscription
		project             *datastore.Project
		endpoint            *datastore.Endpoint
		wantRetryConfig     *datastore.StrategyConfiguration
		wantRateLimitConfig *RateLimitConfig
		wantDisableEndpoint bool
	}{
		{
			name:         "Project Config is primary config",
			subscription: &datastore.Subscription{},
			project: &datastore.Project{
				Config: &datastore.ProjectConfig{
					Strategy: &datastore.StrategyConfiguration{
						Type:       datastore.ExponentialStrategyProvider,
						Duration:   3,
						RetryCount: 4,
					},
					RateLimit: &datastore.RateLimitConfiguration{
						Count:    100,
						Duration: 10,
					},
				},
			},
			endpoint: &datastore.Endpoint{
				RateLimit:         100,
				RateLimitDuration: 600,
			},
			wantRetryConfig: &datastore.StrategyConfiguration{
				Type:       datastore.ExponentialStrategyProvider,
				Duration:   3,
				RetryCount: 4,
			},
			wantRateLimitConfig: &RateLimitConfig{
				Rate:       100,
				BucketSize: 600,
			},
			wantDisableEndpoint: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			evConfig := &EventDeliveryConfig{subscription: tc.subscription, project: tc.project, endpoint: tc.endpoint}

			if tc.wantRetryConfig != nil {
				rc, err := evConfig.RetryConfig()

				assert.Nil(t, err)

				assert.Equal(t, tc.wantRetryConfig.Type, rc.Type)
				assert.Equal(t, tc.wantRetryConfig.Duration, rc.Duration)
				assert.Equal(t, tc.wantRetryConfig.RetryCount, rc.RetryCount)
			}

			if tc.wantRateLimitConfig != nil {
				rlc := evConfig.RateLimitConfig()

				assert.Equal(t, tc.wantRateLimitConfig.Rate, rlc.Rate)
				assert.Equal(t, tc.wantRateLimitConfig.BucketSize, rlc.BucketSize)
			}
		})
	}
}
