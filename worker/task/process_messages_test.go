package task

import (
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/golang/mock/gomock"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestProcessEvents(t *testing.T) {
	tt := []struct {
		name          string
		cfgPath       string
		expectedError error
		msg           *convoy.Event
		dbFn          func(*mocks.MockApplicationRepository, *mocks.MockGroupRepository, *mocks.MockEventRepository)
		nFn           func() func()
	}{
		{
			name:          "Event already sent.",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						AppMetadata: &convoy.AppMetadata{
							Endpoints: []convoy.EndpointMetadata{
								{
									Sent: true,
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)
			},
		},
		{
			name:          "Endpoint is inactive",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						AppMetadata: &convoy.AppMetadata{
							Endpoints: []convoy.EndpointMetadata{
								{
									Status: convoy.InactiveEndpointStatus,
									Sent:   false,
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						Status: convoy.InactiveEndpointStatus,
					}, nil).Times(1)
			},
		},
		{
			name:          "Endpoint does not respond with 2xx",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: &EndpointError{Err: ErrDeliveryAttemptFailed, delay: 20 * time.Second},
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						Data:   []byte(`{"event": "invoice.completed"}`),
						Status: convoy.ScheduledEventStatus,
						Metadata: &convoy.EventMetadata{
							NumTrials:       0,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &convoy.AppMetadata{
							Secret: "aaaaaaaaaaaaaaa",
							Endpoints: []convoy.EndpointMetadata{
								{
									Status:    convoy.ActiveEndpointStatus,
									Sent:      false,
									TargetURL: "https://google.com",
									UID:       "1234567890",
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						Status: convoy.ActiveEndpointStatus,
					}, nil).Times(1)

				m.EXPECT().
					UpdateEventWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						Data:   []byte(`{"event": "invoice.completed"}`),
						Status: convoy.ScheduledEventStatus,
						Metadata: &convoy.EventMetadata{
							NumTrials:       2,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &convoy.AppMetadata{
							Secret: "aaaaaaaaaaaaaaa",
							Endpoints: []convoy.EndpointMetadata{
								{
									Status:    convoy.ActiveEndpointStatus,
									Sent:      false,
									TargetURL: "https://google.com",
									UID:       "1234567890",
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						Status: convoy.ActiveEndpointStatus,
					}, nil).Times(1)

				m.EXPECT().
					UpdateEventWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						Data:   []byte(`{"event": "invoice.completed"}`),
						Status: convoy.ScheduledEventStatus,
						Metadata: &convoy.EventMetadata{
							NumTrials:       2,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &convoy.AppMetadata{
							Secret: "aaaaaaaaaaaaaaa",
							Endpoints: []convoy.EndpointMetadata{
								{
									Status:    convoy.ActiveEndpointStatus,
									Sent:      false,
									TargetURL: "https://google.com",
									UID:       "1234567890",
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						Status: convoy.ActiveEndpointStatus,
					}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Group{
						LogoURL: "",
					}, nil).Times(1)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				m.EXPECT().
					UpdateEventWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						Data:   []byte(`{"event": "invoice.completed"}`),
						Status: convoy.ScheduledEventStatus,
						Metadata: &convoy.EventMetadata{
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &convoy.AppMetadata{
							Secret: "aaaaaaaaaaaaaaa",
							Endpoints: []convoy.EndpointMetadata{
								{
									Status:    convoy.ActiveEndpointStatus,
									Sent:      false,
									TargetURL: "https://google.com",
									UID:       "1234567890",
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						UID:    "1234567890",
						Status: convoy.ActiveEndpointStatus,
					}, nil).Times(1)

				m.EXPECT().
					UpdateEventWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						Data:   []byte(`{"event": "invoice.completed"}`),
						Status: convoy.ScheduledEventStatus,
						Metadata: &convoy.EventMetadata{
							NumTrials:       3,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &convoy.AppMetadata{
							Secret: "aaaaaaaaaaaaaaa",
							Endpoints: []convoy.EndpointMetadata{
								{
									Status:    convoy.ActiveEndpointStatus,
									Sent:      false,
									TargetURL: "https://google.com",
									UID:       "1234567890",
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						UID:    "1234567890",
						Status: convoy.ActiveEndpointStatus,
					}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Group{
						LogoURL: "",
					}, nil).Times(1)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				m.EXPECT().
					UpdateEventWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						Data:   []byte(`{"event": "invoice.completed"}`),
						Status: convoy.ScheduledEventStatus,
						Metadata: &convoy.EventMetadata{
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &convoy.AppMetadata{
							Secret: "aaaaaaaaaaaaaaa",
							Endpoints: []convoy.EndpointMetadata{
								{
									Status:    convoy.ActiveEndpointStatus,
									Sent:      false,
									TargetURL: "https://google.com",
									UID:       "1234567890",
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						UID:    "1234567890",
						Status: convoy.ActiveEndpointStatus,
					}, nil).Times(1)

				m.EXPECT().
					UpdateEventWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msg: &convoy.Event{
				UID: "",
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockEventRepository) {
				m.EXPECT().
					FindEventByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Event{
						Data:   []byte(`{"event": "invoice.completed"}`),
						Status: convoy.ScheduledEventStatus,
						Metadata: &convoy.EventMetadata{
							NumTrials:       4,
							RetryLimit:      3,
							IntervalSeconds: 20,
						},
						AppMetadata: &convoy.AppMetadata{
							Secret: "aaaaaaaaaaaaaaa",
							Endpoints: []convoy.EndpointMetadata{
								{
									Status:    convoy.ActiveEndpointStatus,
									Sent:      false,
									TargetURL: "https://google.com",
									UID:       "1234567890",
								},
							},
						},
					}, nil).Times(1)

				m.EXPECT().
					UpdateStatusOfEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						UID:    "1234567890",
						Status: convoy.PendingEndpointStatus,
					}, nil).Times(1)

				o.EXPECT().
					FetchGroupByID(gomock.Any(), gomock.Any()).
					Return(&convoy.Group{
						LogoURL: "",
					}, nil).Times(1)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				m.EXPECT().
					UpdateEventWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msgRepo := mocks.NewMockEventRepository(ctrl)

			err := config.LoadConfig(tc.cfgPath)
			if err != nil {
				t.Error("Failed to load config file")
			}

			if tc.nFn != nil {
				deferFn := tc.nFn()
				defer deferFn()
			}

			if tc.dbFn != nil {
				tc.dbFn(appRepo, groupRepo, msgRepo)
			}

			processFn := ProcessEvents(appRepo, msgRepo, groupRepo)

			job := queue.Job{
				MsgID: tc.msg.UID,
			}

			err = processFn(&job)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
