package task

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/golang/mock/gomock"
	"github.com/hibiken/asynq"
	"github.com/jarcoal/httpmock"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

var successBody = []byte("event indexed successfully")
var healthCheckBody = []byte(`{}`)

func TestIndexDocument(t *testing.T) {
	tests := []struct {
		name       string
		event      *datastore.Event
		nFn        func(string) func()
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
			nFn: func(url string) func() {
				httpmock.Activate()

				httpmock.RegisterResponder(http.MethodGet, url+"/health",
					httpmock.NewStringResponder(http.StatusOK, string(healthCheckBody)).
						HeaderAdd(http.Header{
							"Content-Type": []string{"application/json"},
						}),
				)

				httpmock.RegisterResponder(http.MethodGet, url+"/collections",
					httpmock.NewStringResponder(http.StatusOK, string(`[]`)).
						HeaderAdd(http.Header{
							"Content-Type": []string{"application/json"},
						}),
				)

				httpmock.RegisterResponder(http.MethodPost, url+"/collections",
					httpmock.NewStringResponder(http.StatusCreated, string(healthCheckBody)).
						HeaderAdd(http.Header{
							"Content-Type": []string{"application/json"},
						}),
				)

				httpmock.RegisterResponderWithQuery(http.MethodPost,
					url+"/collections/project-id-1/documents",
					"action=upsert",
					httpmock.NewStringResponder(http.StatusCreated, string(healthCheckBody)).
						HeaderAdd(http.Header{
							"Content-Type": []string{"application/json"},
						}),
				)

				return func() {
					httpmock.DeactivateAndReset()
				}
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
			nFn: func(url string) func() {
				httpmock.Activate()

				httpmock.RegisterResponder(http.MethodPost, url,
					httpmock.NewStringResponder(http.StatusBadRequest, string(`{}`)))

				return func() {
					httpmock.DeactivateAndReset()
				}
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
				ProjectID: "project-id-1",
				Endpoints: []string{"endpoint-id-1"},
				Data:      []byte(`{}`),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			nFn: func(url string) func() {
				httpmock.Activate()

				httpmock.RegisterResponder(http.MethodPost, url,
					httpmock.NewStringResponder(http.StatusBadRequest, string(`{}`)))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			wantErr:    true,
			wantDelay:  time.Second * 5,
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

			if tt.nFn != nil {
				deferFn := tt.nFn(cfg.Search.Typesense.Host)
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
