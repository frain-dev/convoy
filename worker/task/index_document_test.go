package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/searcher"
	"github.com/frain-dev/convoy/queue"
	"github.com/golang/mock/gomock"
	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

var successBody = []byte("event indexed successfully")
var healthCheckBody = []byte(`{}`)

func TestIndexDocument(t *testing.T) {
	tests := []struct {
		name       string
		event      *datastore.Event
		mFn        func(*testing.T, config.SearchConfiguration) func()
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
			mFn: func(t *testing.T, cfg config.SearchConfiguration) func() {
				return searcher.MockIndexSuccess(t, cfg)
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
			mFn: func(t *testing.T, cfg config.SearchConfiguration) func() {
				return searcher.MockIndexFailed(t, cfg)
			},
			wantErr:    true,
			wantDelay:  time.Second * 5,
			wantErrMsg: "status: 400 response: failed",
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

			cfg, err := config.Get()
			require.NoError(t, err)

			if tt.mFn != nil {
				deferFn := tt.mFn(t, cfg.Search)
				defer deferFn()
			}

			payload, err := json.Marshal(tt.event)
			require.NoError(t, err)

			job := queue.Job{
				ID:      tt.event.UID,
				Payload: payload,
			}

			task := asynq.NewTask(string(convoy.IndexDocument), job.Payload, asynq.Queue(string(convoy.SearchIndexQueue)), asynq.ProcessIn(job.Delay))

			fn := SearchIndex
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
