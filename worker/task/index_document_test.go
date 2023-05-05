package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/queue"
	"github.com/golang/mock/gomock"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func TestIndexDocument(t *testing.T) {
	tests := []struct {
		name       string
		event      *datastore.Event
		mFn        func(args *args)
		wantErr    bool
		wantErrMsg string
		wantDelay  time.Duration
	}{
		{
			name: "should_index_document",
			event: &datastore.Event{
				UID:       ulid.Make().String(),
				EventType: "*",
				SourceID:  "source-id-1",
				ProjectID: "project-id-1",
				Endpoints: []string{"endpoint-id-1"},
				Data:      []byte(`{}`),
				Raw:       "",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mFn: func(args *args) {
				s, _ := args.search.(*mocks.MockSearcher)
				s.EXPECT().Index(gomock.Any(), gomock.Any())
			},
			wantErr: false,
		},
		{
			name: "should_not_index_ducment",
			event: &datastore.Event{
				UID:       ulid.Make().String(),
				EventType: "*",
				SourceID:  "source-id-1",
				ProjectID: "project-id-1",
				Endpoints: []string{"endpoint-id-1"},
				Data:      []byte(`{}`),
				Raw:       "",
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			mFn: func(args *args) {
				srh, _ := args.search.(*mocks.MockSearcher)
				srh.EXPECT().Index(gomock.Any(), gomock.Any()).
					Return(errors.New("[typesense]: 400 Bad Request"))
			},
			wantErr:    true,
			wantDelay:  time.Second * 5,
			wantErrMsg: "[typesense]: 400 Bad Request",
		},
		{
			name: "should_not_index_document_with_missing_project_id",
			event: &datastore.Event{
				UID:       ulid.Make().String(),
				EventType: "*",
				SourceID:  "source-id-1",
				Endpoints: []string{"endpoint-id-1"},
				Data:      []byte(`{}`),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			wantErr:    true,
			wantDelay:  time.Second * 1,
			wantErrMsg: ErrProjectIdFieldIsRequired.Error(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/Config/basic-convoy.json")
			require.NoError(t, err)

			args := provideArgs(ctrl)
			if tt.mFn != nil {
				tt.mFn(args)
			}

			payload, err := json.Marshal(tt.event)
			require.NoError(t, err)

			job := queue.Job{
				ID:      tt.event.UID,
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.IndexDocument), job.Payload, asynq.Queue(string(convoy.SearchIndexQueue)), asynq.ProcessIn(job.Delay))

			indexDocument := &IndexDocument{searchBackend: args.search}
			handler := indexDocument.ProcessTask
			err = handler(context.Background(), task)
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
