package mevent

import (
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideMetaEvent(ctrl *gomock.Controller) *MetaEvent {
	queue := mocks.NewMockQueuer(ctrl)
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	metaEventRepo := mocks.NewMockMetaEventRepository(ctrl)

	return NewMetaEvent(queue, projectRepo, metaEventRepo)
}

func Test_MetaEvent_Run(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	type args struct {
		eventType string
		projectID string
		data      interface{}
	}

	tests := []struct {
		name    string
		args    args
		qFn     func(m *MetaEvent)
		dbFn    func(m *MetaEvent)
		wantErr bool
	}{
		{
			name: "should_create_meta_event",
			args: args{
				eventType: "endpoint.created",
				projectID: "12345",
				data:      &datastore.Endpoint{UID: "123"},
			},
			dbFn: func(m *MetaEvent) {
				projectRepo, _ := m.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.DefaultStrategyConfig,
						MetaEvent: &datastore.MetaEventConfiguration{
							IsEnabled: true,
							EventType: []string{"endpoint.created"},
						},
					},
				}, nil)

				metaEventRepo, _ := m.metaEventRepo.(*mocks.MockMetaEventRepository)
				metaEventRepo.EXPECT().CreateMetaEvent(gomock.Any(), gomock.Any()).Return(nil)
			},
			qFn: func(m *MetaEvent) {
				queue, _ := m.queue.(*mocks.MockQueuer)
				queue.EXPECT().Write(gomock.Any(), gomock.Any(), gomock.Any())
			},
		},

		{
			name: "should_not_create_meta_event_if_project_is_not_subscribed_to_event_type",
			args: args{
				eventType: "endpoint.created",
				projectID: "12345",
				data:      &datastore.Endpoint{UID: "123"},
			},
			dbFn: func(m *MetaEvent) {
				projectRepo, _ := m.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.DefaultStrategyConfig,
						MetaEvent: &datastore.MetaEventConfiguration{
							IsEnabled: true,
							EventType: []string{"endpoint.updated", "endpoint.deleted"},
						},
					},
				}, nil)
			},
		},

		{
			name: "should_not_create_meta_event_if_not_enabled",
			args: args{
				eventType: "endpoint.created",
				projectID: "12345",
				data:      &datastore.Endpoint{UID: "123"},
			},
			dbFn: func(m *MetaEvent) {
				projectRepo, _ := m.projectRepo.(*mocks.MockProjectRepository)
				projectRepo.EXPECT().FetchProjectByID(gomock.Any(), gomock.Any()).Return(&datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Strategy: &datastore.DefaultStrategyConfig,
						MetaEvent: &datastore.MetaEventConfiguration{
							IsEnabled: false,
							EventType: []string{"endpoint.created"},
						},
					},
				}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mE := provideMetaEvent(ctrl)
			if tt.dbFn != nil {
				tt.dbFn(mE)
			}

			if tt.qFn != nil {
				tt.qFn(mE)
			}

			err := mE.Run(tt.args.eventType, tt.args.projectID, tt.args.data)
			require.Nil(t, err)

		})
	}
}
