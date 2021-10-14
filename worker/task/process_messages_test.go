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

func TestProcessMessages(t *testing.T) {
	tt := []struct {
		name          string
		cfgPath       string
		expectedError error
		msg           *convoy.Message
		dbFn          func(*mocks.MockApplicationRepository, *mocks.MockGroupRepository, *mocks.MockMessageRepository)
		nFn           func() func()
	}{
		{
			name:          "Message already sent.",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &convoy.Message{
				AppMetadata: &convoy.AppMetadata{
					Endpoints: []convoy.EndpointMetadata{
						{
							Sent: true,
						},
					},
				},
			},
		},
		{
			name:          "Endpoint is inactive",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &convoy.Message{
				AppMetadata: &convoy.AppMetadata{
					Endpoints: []convoy.EndpointMetadata{
						{
							Status: convoy.InactiveEndpointStatus,
							Sent:   false,
						},
					},
				},
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockMessageRepository) {
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
			msg: &convoy.Message{
				Data:   []byte(`{"event": "invoice.completed"}`),
				Status: convoy.ProcessingMessageStatus,
				Metadata: &convoy.MessageMetadata{
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
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockMessageRepository) {
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						Status: convoy.ActiveEndpointStatus,
					}, nil).Times(1)

				m.EXPECT().
					UpdateMessageWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			name:          "Max retries reached - failed",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &convoy.Message{
				Data:   []byte(`{"event": "invoice.completed"}`),
				Status: convoy.ProcessingMessageStatus,
				Metadata: &convoy.MessageMetadata{
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
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockMessageRepository) {
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
					UpdateMessageWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			name:          "Manual retry failed",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &convoy.Message{
				Data:   []byte(`{"event": "invoice.completed"}`),
				Status: convoy.ProcessingMessageStatus,
				Metadata: &convoy.MessageMetadata{
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
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockMessageRepository) {
				a.EXPECT().
					FindApplicationEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&convoy.Endpoint{
						UID:    "1234567890",
						Status: convoy.PendingEndpointStatus,
					}, nil).Times(1)

				a.EXPECT().
					UpdateApplicationEndpointsStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil).Times(1)

				m.EXPECT().
					UpdateMessageWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			name:          "Manual retry success",
			cfgPath:       "./testdata/Config/basic-convoy.json",
			expectedError: nil,
			msg: &convoy.Message{
				Data:   []byte(`{"event": "invoice.completed"}`),
				Status: convoy.ProcessingMessageStatus,
				Metadata: &convoy.MessageMetadata{
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
			},
			dbFn: func(a *mocks.MockApplicationRepository, o *mocks.MockGroupRepository, m *mocks.MockMessageRepository) {
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
					UpdateMessageWithAttempt(gomock.Any(), gomock.Any(), gomock.Any()).
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
			msgRepo := mocks.NewMockMessageRepository(ctrl)

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

			processFn := ProcessMessages(appRepo, msgRepo, groupRepo)

			job := queue.Job{
				Data: tc.msg,
			}

			err = processFn(&job)

			// Assert.
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
