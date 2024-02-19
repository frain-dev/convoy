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

func provideCreateBroadcastEventService(ctrl *gomock.Controller, de *models.BroadcastEvent, project *datastore.Project) *CreateBroadcastEventService {
	return &CreateBroadcastEventService{
		Queue:          mocks.NewMockQueuer(ctrl),
		BroadcastEvent: de,
		Project:        project,
	}
}

func TestCreateBroadcastEventService_Run(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx          context.Context
		dynamicEvent *models.BroadcastEvent
		g            *datastore.Project
	}
	tests := []struct {
		name        string
		dbFn        func(es *CreateBroadcastEventService)
		args        args
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_create_broadcast_event",
			dbFn: func(es *CreateBroadcastEventService) {
				q, _ := es.Queue.(*mocks.MockQueuer)
				q.EXPECT().Write(convoy.CreateBroadcastEventProcessor, convoy.CreateEventQueue, gomock.Any()).Times(1).Return(nil)
			},
			args: args{
				ctx: ctx,
				dynamicEvent: &models.BroadcastEvent{
					EventType:      "*",
					ProjectID:      "",
					Data:           []byte(`{"name":"daniel"}`),
					CustomHeaders:  nil,
					IdempotencyKey: "",
				},
				g: &datastore.Project{UID: "12345"},
			},
			wantErr: false,
		},
		{
			name: "should_error_for_nil_project",
			dbFn: func(es *CreateBroadcastEventService) {},
			args: args{
				ctx:          ctx,
				dynamicEvent: &models.BroadcastEvent{},
				g:            nil,
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while creating broadcast event - invalid project",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			es := provideCreateBroadcastEventService(ctrl, tc.args.dynamicEvent, tc.args.g)

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
