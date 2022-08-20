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
	"github.com/go-redis/redis_rate/v9"
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
		dbFn          func(*mocks.MockApplicationRepository, *mocks.MockGroupRepository, *mocks.MockEventDeliveryRepository, *mocks.MockRateLimiter, *mocks.MockSubscriptionRepository)
		nFn           func() func()
	}{
		{
			name:          "Event already sent.",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any())
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any())
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any())

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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any())
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.InactiveSubscriptionStatus,
					}, nil)
				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
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

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Application{
						GroupID: "123",
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
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
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: &datastore.StrategyConfiguration{
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
			name:          "Max retries reached - do not disable subscription - failed",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil).Times(2)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Application{
						GroupID: "123",
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       2,
							RetryLimit:      3,
							IntervalSeconds: 20,
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
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: &datastore.StrategyConfiguration{
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
			name:          "Max retries reached - disabled subscription - failed",
			cfgPath:       "./testdata/Config/basic-convoy-disable-endpoint.json",
			expectedError: nil,
			msg: &datastore.EventDelivery{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil).Times(2)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Application{
						GroupID: "123",
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       2,
							RetryLimit:      3,
							IntervalSeconds: 20,
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

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				s.EXPECT().
					UpdateSubscriptionStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil).Times(2)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Application{
						GroupID: "123",
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
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
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: &datastore.StrategyConfiguration{
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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil).Times(2)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Application{
						GroupID: "123",
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
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

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				s.EXPECT().
					UpdateSubscriptionStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil).Times(2)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Application{
						GroupID: "123",
					}, nil)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Status: datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
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
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: &datastore.StrategyConfiguration{
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
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventDeliveryRepository, r *mocks.MockRateLimiter, s *mocks.MockSubscriptionRepository) {
				a.EXPECT().FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Endpoint{
						RateLimit:         10,
						RateLimitDuration: "1m",
					}, nil).Times(2)
				a.EXPECT().FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Application{
						GroupID: "123",
					}, nil).Times(2)
				s.EXPECT().FindSubscriptionByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&datastore.Subscription{
						Status: datastore.ActiveSubscriptionStatus,
					}, nil)

				m.EXPECT().
					FindEventDeliveryByID(gomock.Any(), gomock.Any()).
					Return(&datastore.EventDelivery{
						Status: datastore.ScheduledEventStatus,
						Metadata: &datastore.Metadata{
							Data:            []byte(`{"event": "invoice.completed"}`),
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
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

				m.EXPECT().
					UpdateStatusOfEventDelivery(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&datastore.Group{
						LogoURL: "",
						Config: &datastore.GroupConfig{
							Signature: &datastore.SignatureConfiguration{
								Header: config.SignatureHeaderProvider("X-Convoy-Signature"),
								Hash:   "SHA256",
							},
							Strategy: &datastore.StrategyConfiguration{
								Type:       datastore.LinearStrategyProvider,
								Duration:   60,
								RetryCount: 1,
							},
							DisableEndpoint: true,
						},
					}, nil).Times(1)

				s.EXPECT().UpdateSubscriptionStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
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
			userRepo := mocks.NewMockUserRepository(ctrl)
			cache := mocks.NewMockCache(ctrl)
			rateLimiter := mocks.NewMockRateLimiter(ctrl)
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
				tc.dbFn(appRepo, groupRepo, msgRepo, rateLimiter, subRepo)
			}

			processFn := ProcessEventDelivery(appRepo, msgRepo, groupRepo, rateLimiter, subRepo, q)

			payload := json.RawMessage(tc.msg.UID)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			err = processFn(context.Background(), task)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
