package task

import (
	"context"
	"encoding/json"
	"github.com/frain-dev/convoy/net"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/jarcoal/httpmock"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestProcessEventDelivery(t *testing.T) {
	tt := []struct {
		name          string
		cfgPath       string
		expectedError error
		msg           *datastore.EventDelivery
		dbFn          func(*mocks.MockEndpointRepository, *mocks.MockProjectRepository, *mocks.MockEventDeliveryRepository, *mocks.MockQueuer, *mocks.MockRateLimiter, *mocks.MockDeliveryAttemptsRepository)
		nFn           func() func()
	}{
		{
			name:          "Event already sent.",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
						Status: datastore.SuccessEventStatus,
					}, nil).Times(1)

				endpoint := &datastore.Endpoint{UID: "endpoint-id-1"}
				a.EXPECT().FindEndpointByID(gomock.Any(), "endpoint-id-1", gomock.Any()).Times(1).Return(endpoint, nil)

				project := &datastore.Project{UID: "project-id-1"}
				o.EXPECT().FetchProjectByID(gomock.Any(), "project-id-1").Times(1).Return(project, nil)
			},
		},
		{
			name:          "Endpoint is inactive",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
					}, nil).Times(1)

				m.EXPECT().UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), datastore.DiscardedEventStatus).Times(1).Return(nil)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
		},
		{
			name:          "Endpoint does not respond with 2xx",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status: datastore.ScheduledEventStatus,
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				q.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any())
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
						Status: datastore.ScheduledEventStatus,
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
						Status:         datastore.ScheduledEventStatus,
						URLQueryParams: "name=ref&category=food",
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
						Status: datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				q.EXPECT().
					Write(convoy.NotificationProcessor, convoy.DefaultQueue, gomock.Any()).
					Return(nil).Times(1)
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, q *mocks.MockQueuer, r *mocks.MockRateLimiter, d *mocks.MockDeliveryAttemptsRepository) {
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
						Status: datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							Raw:             `{"event": "invoice.completed"}`,
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
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
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				q.EXPECT().
					Write(convoy.NotificationProcessor, convoy.DefaultQueue, gomock.Any()).
					Return(nil).Times(1)
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

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			cfg, err := config.Get()
			if err != nil {
				t.Errorf("failed to get config: %v", err)
			}

			err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, portalLinkRepo, cache)
			if err != nil {
				t.Errorf("failed to initialize realm chain : %v", err)
			}

			if tc.nFn != nil {
				deferFn := tc.nFn()
				defer deferFn()
			}

			if tc.dbFn != nil {
				tc.dbFn(endpointRepo, projectRepo, msgRepo, q, rateLimiter, attemptsRepo)
			}

			dispatcher, err := net.NewDispatcher("", false)
			require.NoError(t, err)

			processFn := ProcessEventDelivery(endpointRepo, msgRepo, projectRepo, q, rateLimiter, dispatcher, attemptsRepo)

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

			err = processFn(context.Background(), task)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
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
			name: "Subscription Config is primary config",
			subscription: &datastore.Subscription{
				RetryConfig: &datastore.RetryConfiguration{
					Type:       datastore.LinearStrategyProvider,
					Duration:   2,
					RetryCount: 3,
				},
				RateLimitConfig: &datastore.RateLimitConfiguration{
					Count:    100,
					Duration: 1,
				},
			},
			project: &datastore.Project{
				Config: &datastore.ProjectConfig{
					Strategy:  &datastore.DefaultStrategyConfig,
					RateLimit: &datastore.DefaultRateLimitConfig,
				},
			},
			endpoint: &datastore.Endpoint{
				RateLimit:         100,
				RateLimitDuration: 60,
			},
			wantRetryConfig: &datastore.StrategyConfiguration{
				Type:       datastore.LinearStrategyProvider,
				Duration:   2,
				RetryCount: 3,
			},
			wantRateLimitConfig: &RateLimitConfig{
				Rate:       100,
				BucketSize: 60,
			},
			wantDisableEndpoint: true,
		},

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
