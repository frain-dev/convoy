package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/hibiken/asynq"
	"github.com/jarcoal/httpmock"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestProcessEventDelivery(t *testing.T) {
	tt := []struct {
		name          string
		cfgPath       string
		expectedError error
		msg           *datastore.EventDelivery
		dbFn          func(*mocks.MockEndpointRepository, *mocks.MockProjectRepository, *mocks.MockEventDeliveryRepository, *mocks.MockSubscriptionRepository, *mocks.MockQueuer)
		nFn           func() func()
	}{
		{
			name:          "Event already sent.",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{RetryConfig: &datastore.DefaultRetryConfig}, nil)

				o.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						Status: datastore.SuccessEventStatus,
					}, nil).Times(1)
			},
		},
		{
			name:          "Endpoint is inactive",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
						Status:            datastore.InactiveEndpointStatus,
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				o.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{Config: &datastore.ProjectConfig{
					RateLimit: &datastore.DefaultRateLimitConfig,
					Strategy:  &datastore.DefaultStrategyConfig,
				}}, nil)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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
			expectedError: &EndpointError{Err: ErrDeliveryAttemptFailed, delay: 20 * time.Second},
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID:         "123",
						RateLimit:         10,
						RateLimitDuration: "1m",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						Status: datastore.ActiveEndpointStatus,
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			name:          "Max retries reached - disabled endpoint - failed",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: "1m",
						ProjectID:         "123",
						Status:            datastore.ActiveEndpointStatus,
					}, nil)

				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: "1m",
						Status:            datastore.ActiveEndpointStatus,
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				a.EXPECT().
					UpdateEndpointStatus(gomock.Any(), gomock.Any(), gomock.Any(), datastore.InactiveEndpointStatus).
					Return(nil).Times(1)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: "1m",
						Status:            datastore.ActiveEndpointStatus,
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						TargetURL: "https://google.com?source=giphy",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: "1m",
						Status:            datastore.ActiveEndpointStatus,
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID: "123",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						RateLimitDuration: "1m",
						Status:            datastore.ActiveEndpointStatus,
					}, nil).Times(1)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID:    "123",
						SupportEmail: "test@gmail.com",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						Status:            datastore.ActiveEndpointStatus,
						RateLimitDuration: "1m",
					}, nil).Times(1)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockEndpointRepository, o *mocks.MockProjectRepository, m *mocks.MockEventDeliveryRepository, s *mocks.MockSubscriptionRepository, q *mocks.MockQueuer) {
				a.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						ProjectID:    "123",
						SupportEmail: "test@gmail.com",
						Secrets: []datastore.Secret{
							{Value: "secret"},
						},
						RateLimit:         10,
						TargetURL:         "https://google.com",
						RateLimitDuration: "1m",
						Status:            datastore.PendingEndpointStatus,
					}, nil).Times(1)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any(), gomock.Any()).
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

				m.EXPECT().
					UpdateEventDeliveryWithAttempt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			subRepo := mocks.NewMockSubscriptionRepository(ctrl)
			q := mocks.NewMockQueuer(ctrl)

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			cfg, err := config.Get()
			if err != nil {
				t.Errorf("failed to get config: %v", err)
			}

			err = realm_chain.Init(&cfg.Auth, apiKeyRepo, userRepo, cache)
			if err != nil {
				t.Errorf("failed to initialize realm chain : %v", err)
			}

			if tc.nFn != nil {
				deferFn := tc.nFn()
				defer deferFn()
			}

			if tc.dbFn != nil {
				tc.dbFn(endpointRepo, projectRepo, msgRepo, subRepo, q)
			}

			processFn := ProcessEventDelivery(endpointRepo, msgRepo, projectRepo, subRepo, q)

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
		wantRetryConfig     *datastore.StrategyConfiguration
		wantRateLimitConfig *datastore.RateLimitConfiguration
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
			wantRetryConfig: &datastore.StrategyConfiguration{
				Type:       datastore.LinearStrategyProvider,
				Duration:   2,
				RetryCount: 3,
			},
			wantRateLimitConfig: &datastore.RateLimitConfiguration{
				Count:    100,
				Duration: 1,
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
			wantRetryConfig: &datastore.StrategyConfiguration{
				Type:       datastore.ExponentialStrategyProvider,
				Duration:   3,
				RetryCount: 4,
			},
			wantRateLimitConfig: &datastore.RateLimitConfiguration{
				Count:    100,
				Duration: 10,
			},
			wantDisableEndpoint: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			evConfig := &EventDeliveryConfig{subscription: tc.subscription, project: tc.project}

			if tc.wantRetryConfig != nil {
				rc, err := evConfig.retryConfig()

				assert.Nil(t, err)

				assert.Equal(t, tc.wantRetryConfig.Type, rc.Type)
				assert.Equal(t, tc.wantRetryConfig.Duration, rc.Duration)
				assert.Equal(t, tc.wantRetryConfig.RetryCount, rc.RetryCount)
			}

			if tc.wantRateLimitConfig != nil {
				rlc := evConfig.rateLimitConfig()

				assert.Equal(t, tc.wantRateLimitConfig.Count, rlc.Count)
				assert.Equal(t, tc.wantRateLimitConfig.Duration, rlc.Duration)
			}
		})
	}
}
