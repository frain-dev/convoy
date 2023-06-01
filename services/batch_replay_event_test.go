package services

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
)

func provideBatchReplayEventService(ctrl *gomock.Controller, f *datastore.Filter) *BatchReplayEventService {
	return &BatchReplayEventService{
		EndpointRepo: mocks.NewMockEndpointRepository(ctrl),
		Queue:        mocks.NewMockQueuer(ctrl),
		EventRepo:    mocks.NewMockEventRepository(ctrl),
		Filter:       f,
	}
}

func TestBatchReplayEventService_Run(t *testing.T) {
	type fields struct {
		EndpointRepo datastore.EndpointRepository
		Queue        queue.Queuer
		EventRepo    datastore.EventRepository
		Filter       *datastore.Filter
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		want1   int
		wantErr bool
	}{
		{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &BatchReplayEventService{
				EndpointRepo: tt.fields.EndpointRepo,
				Queue:        tt.fields.Queue,
				EventRepo:    tt.fields.EventRepo,
				Filter:       tt.fields.Filter,
			}
			got, got1, err := e.Run(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchReplayEventService.Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("BatchReplayEventService.Run() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("BatchReplayEventService.Run() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
