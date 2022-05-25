package task

import (
	"testing"
	"time"

	"github.com/frain-dev/convoy/auth/realm_chain"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/disq"
	"github.com/go-redis/redis_rate/v9"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/golang/mock/gomock"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestProcessEventDelivery(t *testing.T) {
	tt := []struct {
		name          string
		cfgPath       string
		expectedError error
		msg           *datastore.EventDelivery
		dbFn          func(*mocks.MockApplicationRepository, *mocks.MockGroupRepository, *mocks.MockEventDeliveryRepository, *mocks.MockRateLimiter)
		nFn           func() func()
	}{
		{
			name:          "Event already sent.",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &datastore.AppMetadata{},
						EndpointMetadata: &datastore.EndpointMetadata{
							Status: datastore.InactiveEndpointStatus,
						},
					}, nil).Times(1)

				//ns.EXPECT()

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						Status: datastore.InactiveEndpointStatus,
					}, nil).Times(1)
			},
		},
		{
			name:          "Endpoint does not respond with 2xx",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: &disq.Error{Err: ErrDeliveryAttemptFailed, Delay: 20 * time.Second},
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						AppMetadata: &datastore.AppMetadata{},
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							Secret:    "aaaaaaaaaaaaaaa",
							Status:    datastore.ActiveEndpointStatus,
							Sent:      false,
							TargetURL: "https://google.com",
							UID:       "1234567890",
						},
						Status: datastore.ScheduledEventStatus,
					}, nil).Times(1)

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

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
			name:          "Max retries reached - do not disable endpoint - failed",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						AppMetadata: &datastore.AppMetadata{},
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       2,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							Secret:    "aaaaaaaaaaaaaaa",
							Status:    datastore.ActiveEndpointStatus,
							Sent:      false,
							TargetURL: "https://google.com",
							UID:       "1234567890",
						},
						Status: datastore.ScheduledEventStatus,
					}, nil).Times(1)

				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Return(&datastore.Application{}, nil)

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: false,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

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
			name:          "Max retries reached - disable endpoint - failed",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						AppMetadata: &datastore.AppMetadata{
							SupportEmail: "aaaaaaaaaaaaaaa",
						},
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       2,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							Secret:    "aaaaaaaaaaaaaaa",
							Status:    datastore.ActiveEndpointStatus,
							Sent:      false,
							TargetURL: "https://google.com",
							UID:       "1234567890",
						},
						Status: datastore.ScheduledEventStatus,
					}, nil).Times(1)

				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Return(&datastore.Application{}, nil)

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

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
			name:          "Manual retry - no disable endpoint - failed",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						AppMetadata: &datastore.AppMetadata{},
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							Secret:    "aaaaaaaaaaaaaaa",
							Status:    datastore.ActiveEndpointStatus,
							Sent:      false,
							TargetURL: "https://google.com",
							UID:       "1234567890",
						},
						Status: datastore.ScheduledEventStatus,
					}, nil).Times(1)

				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Return(&datastore.Application{}, nil)

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: datastore.StrategyConfiguration{
								Type:       datastore.StrategyProvider("default"),
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: false,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						UID:    "1234567890",
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						AppMetadata: &datastore.AppMetadata{
							SupportEmail: "aaaaaaaaaaaaaaa",
						},
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							Secret:    "aaaaaaaaaaaaaaa",
							Status:    datastore.ActiveEndpointStatus,
							Sent:      false,
							TargetURL: "https://google.com",
							UID:       "1234567890",
						},
						Status: datastore.ScheduledEventStatus,
					}, nil).Times(1)

				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Return(&datastore.Application{}, nil)

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						UID:    "1234567890",
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

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
			name:          "Manual retry - no disable endpoint - success",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Status:      datastore.ScheduledEventStatus,
						AppMetadata: &datastore.AppMetadata{},
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							Secret:    "aaaaaaaaaaaaaaa",
							Status:    datastore.ActiveEndpointStatus,
							Sent:      false,
							TargetURL: "https://google.com",
							UID:       "1234567890",
						},
					}, nil).Times(1)

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Return(&datastore.Application{}, nil)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: false,
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						UID:    "1234567890",
						Status: datastore.ActiveEndpointStatus,
					}, nil).Times(1)

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
			name:          "Manual retry - disable endpoint - success",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter) {
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						AppMetadata: &datastore.AppMetadata{
							SupportEmail: "aaaaaaaaaaaaaaa",
						},
						Status: datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						EndpointMetadata: &datastore.EndpointMetadata{
							Secret:    "aaaaaaaaaaaaaaa",
							Status:    datastore.ActiveEndpointStatus,
							Sent:      false,
							TargetURL: "https://google.com",
							UID:       "1234567890",
						},
					}, nil).Times(1)

				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).Times(2).Return(&datastore.Application{}, nil)

				r.EXPECT().ShouldAllow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				r.EXPECT().Allow(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&redis_rate.Result{
					Limit:     redis_rate.PerMinute(10),
					Allowed:   10,
					Remaining: 10,
				}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						UID:    "1234567890",
						Status: datastore.PendingEndpointStatus,
					}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

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
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			groupRepo := mocks.NewMockGroupRepository(ctrl)
			appRepo := mocks.NewMockApplicationRepository(ctrl)
			msgRepo := mocks.NewMockEventDeliveryRepository(ctrl)
			apiKeyRepo := mocks.NewMockAPIKeyRepository(ctrl)
			rateLimiter := mocks.NewMockRateLimiter(ctrl)

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			cfg, err := config.Get()
			if err != nil {
				t.Errorf("failed to get config: %v", err)
			}

			err = realm_chain.Init(&cfg.Auth, apiKeyRepo)
			if err != nil {
				t.Errorf("failed to initialize realm chain : %v", err)
			}

			if tc.nFn != nil {
				deferFn := tc.nFn()
				defer deferFn()
			}

			if tc.dbFn != nil {
				tc.dbFn(appRepo, groupRepo, msgRepo, rateLimiter)
			}

			processFn := ProcessEventDelivery(appRepo, msgRepo, groupRepo, rateLimiter)

			job := queue.Job{
				ID: tc.msg.UID,
			}

			err = processFn(&job)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
