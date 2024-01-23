package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideCreateDynamicEventService(ctrl *gomock.Controller, de *models.DynamicEvent, project *datastore.Project) *CreateDynamicEventService {
	return &CreateDynamicEventService{
		Queue:        mocks.NewMockQueuer(ctrl),
		DynamicEvent: de,
		Project:      project,
	}
}

func TestCreateDynamicEventService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx          context.Context
		dynamicEvent *models.DynamicEvent
		g            *datastore.Project
	}
	tests := []struct {
		name        string
		dbFn        func(es *CreateDynamicEventService)
		args        args
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_dynamic_event",
			dbFn: func(es *CreateDynamicEventService) {
				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.CreateDynamicEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				dynamicEvent: &models.DynamicEvent{
					URL:            "https://google.com",
					Secret:         "abc",
					EventTypes:     []string{"*"},
					Data:           []byte(`{"name":"daniel"}`),
					EventType:      "*",
					IdempotencyKey: "",
				},
				g: &datastore.Project{UID: "12345"},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_nil_project",
			dbFn: func(es *CreateDynamicEventService) {},
			args: args{
				ctx:          ctx,
				dynamicEvent: &models.DynamicEvent{},
				g:            nil,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while creating dynamic event - invalid project",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideCreateDynamicEventService(ctrl, tc.args.dynamicEvent, tc.args.g)

			if tc.dbFn != nil {
				tc.dbFn(es)
			}

			err = es.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.Error())
				return
			}

			require.Nil(t, err)
		})
	}
}
