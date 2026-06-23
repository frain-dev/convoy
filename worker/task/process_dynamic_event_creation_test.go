package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

func TestProcessDynamicEventCreation(t *testing.T) {
	tests := []struct {
		name         string
		dynamicEvent *models.DynamicEvent
		dbFn         func(args *testArgs)
		wantErr      bool
		wantErrMsg   string
		wantDelay    time.Duration
	}{
		{
			name: "should_create_dynamic_event",
			dynamicEvent: &models.DynamicEvent{
				JobID:          "123:1234567890",
				URL:            "https://google.com",
				Secret:         "1234",
				EventTypes:     []string{"*"},
				Data:           []byte(`{"name":"daniel"}`),
				ProjectID:      "project-id-1",
				EventType:      "*",
				IdempotencyKey: "idem-key-1",
			},
			dbFn: func(args *testArgs) {
				project := &datastore.Project{
					UID:  "project-id-1",
					Type: datastore.OutgoingProject,
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       datastore.LinearStrategyProvider,
							Duration:   10,
							RetryCount: 3,
						},
					},
				}

				g, _ := args.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "project-id-1").Times(1).Return(
					project,
					nil,
				)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventsByIdempotencyKey(gomock.Any(), "project-id-1", "idem-key-1").Times(1).Return(false, nil)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

			},
			wantErr: false,
		},
		{
			name: "should_create_new_endpoint_and_subscription_for_dynamic_event",
			dynamicEvent: &models.DynamicEvent{
				JobID:     "123:1234567890",
				URL:       "https://google.com",
				Secret:    "1234",
				Data:      []byte(`{"name":"daniel"}`),
				ProjectID: "project-id-1",
				EventType: "*",
			},
			dbFn: func(args *testArgs) {
				project := &datastore.Project{
					UID:  "project-id-1",
					Type: datastore.OutgoingProject,
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.StrategyConfiguration{
							Type:       datastore.LinearStrategyProvider,
							Duration:   10,
							RetryCount: 3,
						},
					},
				}

				g, _ := args.projectRepo.(*mocks.MockProjectRepository)
				g.EXPECT().FetchProjectByID(gomock.Any(), "project-id-1").Times(1).Return(
					project,
					nil,
				)

				e, _ := args.eventRepo.(*mocks.MockEventRepository)
				e.EXPECT().CreateEvent(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				q, _ := args.eventQueue.(*mocks.MockQueuer)
				q.EXPECT().Write(gomock.Any(), convoy.MatchEventSubscriptionsProcessor, convoy.EventWorkflowQueue, gomock.Any()).Times(1).Return(nil)

			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			args := provideArgs(ctrl)

			if tt.dbFn != nil {
				tt.dbFn(args)
			}

			payload, err := msgpack.EncodeMsgPack(tt.dynamicEvent)
			require.NoError(t, err)

			job := queue.Job{
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.EventProcessor), job.Payload, asynq.Queue(string(convoy.EventQueue)), asynq.ProcessIn(job.Delay))

			deps := EventProcessorDeps{
				EndpointRepo:       args.endpointRepo,
				EventRepo:          args.eventRepo,
				ProjectRepo:        args.projectRepo,
				EventQueue:         args.eventQueue,
				SubRepo:            args.subRepo,
				FilterRepo:         args.filterRepo,
				Licenser:           args.licenser,
				OAuth2TokenService: args.oauth2TokenService,
			}
			fn := ProcessDynamicEventCreation(deps)
			err = fn(context.Background(), task)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*EndpointError).Error())
				require.Equal(t, tt.wantDelay, err.(*EndpointError).Delay())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestFindDynamicSubscription_UsesAtomicDynamicFindOrCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	project := &datastore.Project{UID: "project-id-1"}
	endpoint := &datastore.Endpoint{UID: "endpoint-id-1"}
	existing := &datastore.Subscription{
		UID:        "subscription-id-1",
		ProjectID:  project.UID,
		EndpointID: endpoint.UID,
		Name:       "dynamic-subscription-endpoint-id-1",
	}

	subRepo := mocks.NewMockSubscriptionRepository(ctrl)
	subRepo.EXPECT().FindSubscriptionsByEndpointID(gomock.Any(), project.UID, endpoint.UID).
		Return([]datastore.Subscription{}, nil)
	subRepo.EXPECT().FindOrCreateDynamicSubscription(gomock.Any(), project.UID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, sub *datastore.Subscription) (*datastore.Subscription, error) {
			require.Equal(t, "dynamic-subscription-endpoint-id-1", sub.Name)
			return existing, nil
		})
	subRepo.EXPECT().UpdateSubscription(gomock.Any(), project.UID, existing).
		DoAndReturn(func(_ context.Context, _ string, sub *datastore.Subscription) error {
			require.ElementsMatch(t, []string{"payment.qrcode"}, sub.FilterConfig.EventTypes)
			return nil
		})

	got, err := findDynamicSubscription(context.Background(), &models.DynamicEvent{EventTypes: []string{"payment.qrcode"}}, subRepo, project, endpoint)
	require.NoError(t, err)
	require.Equal(t, existing.UID, got.UID)
}

func TestFindEndpoint_TemplateMatching(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T, args EventChannelArgs, project *datastore.Project)
	}{
		{
			name: "exact match wins before template lookup",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				endpoint := &datastore.Endpoint{UID: "endpoint-1", ProjectID: project.UID, Url: "https://example.com/orders/123/callback"}

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, endpoint.Url).Return(endpoint, nil)

				got, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: endpoint.Url})
				require.NoError(t, err)
				require.Equal(t, endpoint.UID, got.UID)
			},
		},
		{
			name: "dynamic URL cannot be unresolved template",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				_, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: "https://example.com/orders/{reference}/callback"})
				require.Error(t, err)
				require.Contains(t, err.Error(), "dynamic event URL must be concrete")
			},
		},
		{
			name: "template match reuses configured endpoint",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				concreteURL := "https://example.com/orders/123/callback"
				templatedEndpoint := datastore.Endpoint{UID: "endpoint-template", ProjectID: project.UID, Url: "https://example.com/orders/{reference}/callback"}

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, concreteURL).Return(nil, datastore.ErrEndpointNotFound)
				repo.EXPECT().FindEndpointsWithURLTemplates(gomock.Any(), project.UID).
					Return([]datastore.Endpoint{templatedEndpoint}, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().EndpointURLTemplates().Return(true)

				args.earlyAdopterFeatureFetcher = &mocks.MockEarlyAdopterFeatureFetcher{
					FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
						require.Equal(t, project.OrganisationID, orgID)
						require.Equal(t, string(fflag.EndpointURLTemplates), featureKey)
						return &fflag.EarlyAdopterFeatureInfo{Enabled: true}, nil
					},
				}

				got, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: concreteURL})
				require.NoError(t, err)
				require.Equal(t, templatedEndpoint.UID, got.UID)
			},
		},
		{
			name: "feature disabled falls back to auto create",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				concreteURL := "https://example.com/orders/123/callback"

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, concreteURL).Return(nil, datastore.ErrEndpointNotFound)
				repo.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), project.UID).DoAndReturn(func(_ context.Context, endpoint *datastore.Endpoint, _ string) error {
					require.Equal(t, concreteURL, endpoint.Url)
					return nil
				})

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().EndpointURLTemplates().Return(false)

				got, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: concreteURL, Secret: "secret"})
				require.NoError(t, err)
				require.Equal(t, concreteURL, got.Url)
			},
		},
		{
			name: "feature lookup error fails closed when valid templates exist",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				concreteURL := "https://example.com/orders/123/callback"
				templates := []datastore.Endpoint{
					{UID: "endpoint-template", ProjectID: project.UID, Url: "https://example.com/orders/{reference}/callback"},
				}

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, concreteURL).Return(nil, datastore.ErrEndpointNotFound)
				repo.EXPECT().FindEndpointsWithURLTemplates(gomock.Any(), project.UID).Return(templates, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().EndpointURLTemplates().Return(true)

				args.earlyAdopterFeatureFetcher = &mocks.MockEarlyAdopterFeatureFetcher{
					FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
						return nil, errors.New("feature store unavailable")
					},
				}

				_, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: concreteURL})
				require.Error(t, err)
				require.Contains(t, err.Error(), "endpoint URL template feature lookup failed")
			},
		},
		{
			name: "feature lookup error auto creates when no valid templates exist",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				concreteURL := "https://example.com/orders/123/callback"

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, concreteURL).Return(nil, datastore.ErrEndpointNotFound)
				repo.EXPECT().FindEndpointsWithURLTemplates(gomock.Any(), project.UID).Return([]datastore.Endpoint{}, nil)
				repo.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), project.UID).DoAndReturn(func(_ context.Context, endpoint *datastore.Endpoint, _ string) error {
					require.Equal(t, concreteURL, endpoint.Url)
					return nil
				})

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().EndpointURLTemplates().Return(true)

				args.earlyAdopterFeatureFetcher = &mocks.MockEarlyAdopterFeatureFetcher{
					FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
						return nil, errors.New("feature store unavailable")
					},
				}

				got, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: concreteURL, Secret: "secret"})
				require.NoError(t, err)
				require.Equal(t, concreteURL, got.Url)
			},
		},
		{
			name: "invalid brace candidates do not block auto create",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				concreteURL := "https://example.com/orders/123/callback"
				invalidTemplateCandidate := datastore.Endpoint{UID: "endpoint-invalid-template", ProjectID: project.UID, Url: "https://example.com/orders/{bad-token}/callback"}

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, concreteURL).Return(nil, datastore.ErrEndpointNotFound)
				repo.EXPECT().FindEndpointsWithURLTemplates(gomock.Any(), project.UID).Return([]datastore.Endpoint{invalidTemplateCandidate}, nil)
				repo.EXPECT().CreateEndpoint(gomock.Any(), gomock.Any(), project.UID).DoAndReturn(func(_ context.Context, endpoint *datastore.Endpoint, _ string) error {
					require.Equal(t, concreteURL, endpoint.Url)
					return nil
				})

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().EndpointURLTemplates().Return(true)

				args.earlyAdopterFeatureFetcher = &mocks.MockEarlyAdopterFeatureFetcher{
					FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
						return &fflag.EarlyAdopterFeatureInfo{Enabled: true}, nil
					},
				}

				got, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: concreteURL, Secret: "secret"})
				require.NoError(t, err)
				require.Equal(t, concreteURL, got.Url)
			},
		},
		{
			name: "template miss fails closed when templates are configured",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				concreteURL := "https://example.com/invoices/123/callback"
				templates := []datastore.Endpoint{
					{UID: "endpoint-template", ProjectID: project.UID, Url: "https://example.com/orders/{reference}/callback"},
				}

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, concreteURL).Return(nil, datastore.ErrEndpointNotFound)
				repo.EXPECT().FindEndpointsWithURLTemplates(gomock.Any(), project.UID).Return(templates, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().EndpointURLTemplates().Return(true)

				args.earlyAdopterFeatureFetcher = &mocks.MockEarlyAdopterFeatureFetcher{
					FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
						return &fflag.EarlyAdopterFeatureInfo{Enabled: true}, nil
					},
				}

				_, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: concreteURL})
				require.Error(t, err)
				require.Contains(t, err.Error(), "dynamic URL does not match any configured endpoint URL template")
			},
		},
		{
			name: "overlapping template matches fail closed",
			fn: func(t *testing.T, args EventChannelArgs, project *datastore.Project) {
				concreteURL := "https://example.com/orders/123/callback"
				templates := []datastore.Endpoint{
					{UID: "endpoint-template-1", ProjectID: project.UID, Url: "https://example.com/orders/{reference}/callback"},
					{UID: "endpoint-template-2", ProjectID: project.UID, Url: "https://example.com/orders/{transaction_id}/callback"},
				}

				repo, _ := args.endpointRepo.(*mocks.MockEndpointRepository)
				repo.EXPECT().FindEndpointByTargetURL(gomock.Any(), project.UID, concreteURL).Return(nil, datastore.ErrEndpointNotFound)
				repo.EXPECT().FindEndpointsWithURLTemplates(gomock.Any(), project.UID).Return(templates, nil)

				licenser, _ := args.licenser.(*mocks.MockLicenser)
				licenser.EXPECT().EndpointURLTemplates().Return(true)

				args.earlyAdopterFeatureFetcher = &mocks.MockEarlyAdopterFeatureFetcher{
					FetchEarlyAdopterFeatureFunc: func(ctx context.Context, orgID, featureKey string) (*fflag.EarlyAdopterFeatureInfo, error) {
						return &fflag.EarlyAdopterFeatureInfo{Enabled: true}, nil
					},
				}

				_, err := findEndpoint(context.Background(), project, args, &models.DynamicEvent{URL: concreteURL})
				require.Error(t, err)
				require.Contains(t, err.Error(), "multiple endpoint URL templates match dynamic URL")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			args := EventChannelArgs{
				endpointRepo:               mocks.NewMockEndpointRepository(ctrl),
				licenser:                   mocks.NewMockLicenser(ctrl),
				featureFlag:                fflag.NoopFflag(),
				featureFlagFetcher:         mocks.NewMockFeatureFlagFetcher(),
				earlyAdopterFeatureFetcher: mocks.NewMockEarlyAdopterFeatureFetcherWithMTLSEnabled(),
				logger:                     log.New("convoy", log.LevelError),
			}
			project := &datastore.Project{UID: "project-id-1", OrganisationID: "org-id-1"}

			tt.fn(t, args, project)
		})
	}
}
