package services

import (
	"context"
	"errors"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
)

func provideMetaEventService(ctrl *gomock.Controller) *MetaEventService {
	queue := mocks.NewMockQueuer(ctrl)
	metaEventRepo := mocks.NewMockMetaEventRepository(ctrl)

	return &MetaEventService{queue, metaEventRepo}
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
				queue.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
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
