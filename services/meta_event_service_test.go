package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideMetaEventService(ctrl *gomock.Controller) *MetaEventService {
	queue := mocks.NewMockQueuer(ctrl)
	metaEventRepo := mocks.NewMockMetaEventRepository(ctrl)
	mockLogger := mocks.NewMockLogger(ctrl)

	return &MetaEventService{Queue: queue, MetaEventRepo: metaEventRepo, Logger: mockLogger}
}

func TestMetaEventService(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		metaEvent *datastore.MetaEvent
	}

	tests := []struct {
		name    string
		args    args
		mockDep func(m *MetaEventService)
		wantErr bool
	}{
		{
			name: "should_resend_meta_event",
			args: args{
				ctx: ctx,
				metaEvent: &datastore.MetaEvent{
					UID: "123",
				},
			},
			mockDep: func(m *MetaEventService) {
				metaEventRepo, _ := m.MetaEventRepo.(*mocks.MockMetaEventRepository)
				metaEventRepo.EXPECT().UpdateMetaEvent(gomock.Any(), gomock.Any(), gomock.Any())

				queue, _ := m.Queue.(*mocks.MockQueuer)
				queue.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any())
			},
		},

		{
			name: "should_fail_to_resend_meta_event",
			args: args{
				ctx:       ctx,
				metaEvent: &datastore.MetaEvent{UID: "123"},
			},
			mockDep: func(m *MetaEventService) {
				metaEventRepo, _ := m.MetaEventRepo.(*mocks.MockMetaEventRepository)
				metaEventRepo.EXPECT().UpdateMetaEvent(
					gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("failed to update meta event"))

				ml, _ := m.Logger.(*mocks.MockLogger)
				ml.EXPECT().ErrorContext(gomock.Any(), "failed to update meta event", "error", gomock.Any()).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			metaEventService := provideMetaEventService(ctrl)

			// Arrange Expectations
			if tt.mockDep != nil {
				tt.mockDep(metaEventService)
			}

			err := metaEventService.Run(tt.args.ctx, tt.args.metaEvent)
			if tt.wantErr {
				require.NotNil(t, err)
				return
			}

			require.Nil(t, err)
		})
	}
}
