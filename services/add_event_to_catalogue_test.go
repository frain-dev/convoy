package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/mocks"

	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideAddEventToCatalogueService(ctrl *gomock.Controller, p *datastore.Project, e *models.AddEventToCatalogue) *AddEventToCatalogueService {
	return &AddEventToCatalogueService{
		CatalogueRepo:  mocks.NewMockEventCatalogueRepository(ctrl),
		EventRepo:      mocks.NewMockEventRepository(ctrl),
		CatalogueEvent: e,
		Project:        p,
	}
}

func TestAddEventToCatalogueService_Run(t *testing.T) {
	type args struct {
		ctx            context.Context
		CatalogueEvent *models.AddEventToCatalogue
		Project        *datastore.Project
	}
	tests := []struct {
		name       string
		dbFn       func(es *AddEventToCatalogueService)
		args       args
		want       *datastore.EventCatalogue
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_add_event_to_catalogue",
			dbFn: func(es *AddEventToCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(
					&datastore.EventCatalogue{
						ProjectID: "project_1",
						Type:      datastore.EventsDataCatalogueType,
						Events: datastore.EventDataCatalogues{
							{
								Name:    "invoice.created",
								EventID: "abc",
								Data:    []byte(`{"id":"09323hj"}`),
							},
						},
					},
					nil,
				)

				e, _ := es.EventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventByID(gomock.Any(), "project_1", "1234").Times(1).Return(
					&datastore.Event{
						UID:       "1234",
						ProjectID: "project_1",
						Data:      []byte(`{"name":"daniel"}`),
					},
					nil,
				)

				cr.EXPECT().UpdateEventCatalogue(gomock.Any(), &datastore.EventCatalogue{
					ProjectID: "project_1",
					Type:      datastore.EventsDataCatalogueType,
					Events: datastore.EventDataCatalogues{
						{
							Name:    "invoice.created",
							EventID: "abc",
							Data:    []byte(`{"id":"09323hj"}`),
						},
						{
							EventID: "1234",
							Name:    "invoice.paid",
							Data:    []byte(`{"name":"daniel"}`),
						},
					},
				}).Times(1).Return(nil)
			},
			args: args{
				ctx: context.Background(),
				CatalogueEvent: &models.AddEventToCatalogue{
					EventID: "1234",
					Name:    "invoice.paid",
				},
				Project: &datastore.Project{UID: "project_1", Type: datastore.OutgoingProject},
			},
			want: &datastore.EventCatalogue{Events: datastore.EventDataCatalogues{
				{
					Name:    "invoice.created",
					EventID: "abc",
					Data:    []byte(`{"id":"09323hj"}`),
				},
				{
					EventID: "1234",
					Name:    "invoice.paid",
					Data:    []byte(`{"name":"daniel"}`),
				},
			}},
		},
		{
			name: "should_create_new_catalogue_and_add_event",
			dbFn: func(es *AddEventToCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(nil, datastore.ErrCatalogueNotFound)

				e, _ := es.EventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventByID(gomock.Any(), "project_1", "1234").Times(1).Return(
					&datastore.Event{
						UID:       "1234",
						ProjectID: "project_1",
						Data:      []byte(`{"name":"daniel"}`),
					},
					nil,
				)
				cr.EXPECT().CreateEventCatalogue(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
			args: args{
				ctx: context.Background(),
				CatalogueEvent: &models.AddEventToCatalogue{
					EventID: "1234",
					Name:    "invoice.paid",
				},
				Project: &datastore.Project{UID: "project_1", Type: datastore.OutgoingProject},
			},
			want: &datastore.EventCatalogue{Events: datastore.EventDataCatalogues{
				{
					EventID: "1234",
					Name:    "invoice.paid",
					Data:    []byte(`{"name":"daniel"}`),
				},
			}},
		},

		{
			name: "should_fail_to_find_catalogue",
			dbFn: func(es *AddEventToCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(nil, errors.New("failed"))
			},
			args: args{
				ctx: context.Background(),
				CatalogueEvent: &models.AddEventToCatalogue{
					EventID: "1234",
					Name:    "invoice.paid",
				},
				Project: &datastore.Project{UID: "project_1", Type: datastore.OutgoingProject},
			},
			wantErr:    true,
			wantErrMsg: "unable to fetch event catalogue",
		},
		{
			name: "should_fail_to_find_event",
			dbFn: func(es *AddEventToCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(nil, datastore.ErrCatalogueNotFound)

				e, _ := es.EventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventByID(gomock.Any(), "project_1", "1234").Times(1).Return(
					nil, datastore.ErrEventNotFound,
				)
			},
			args: args{
				ctx: context.Background(),
				CatalogueEvent: &models.AddEventToCatalogue{
					EventID: "1234",
					Name:    "invoice.paid",
				},
				Project: &datastore.Project{UID: "project_1", Type: datastore.OutgoingProject},
			},
			wantErr:    true,
			wantErrMsg: "failed to fetch event",
		},
		{
			name: "should_fail_for_wrong_catalogue_type",
			dbFn: func(es *AddEventToCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(
					&datastore.EventCatalogue{
						ProjectID: "project_1",
						Type:      datastore.OpenAPICatalogueType,
					},
					nil,
				)

				e, _ := es.EventRepo.(*mocks.MockEventRepository)
				e.EXPECT().FindEventByID(gomock.Any(), "project_1", "1234").Times(1).Return(
					&datastore.Event{
						UID:       "1234",
						ProjectID: "project_1",
						Data:      []byte(`{"name":"daniel"}`),
					},
					nil,
				)
			},
			args: args{
				ctx: context.Background(),
				CatalogueEvent: &models.AddEventToCatalogue{
					EventID: "1234",
					Name:    "invoice.paid",
				},
				Project: &datastore.Project{UID: "project_1", Type: datastore.OutgoingProject},
			},
			wantErr:    true,
			wantErrMsg: "you cannot add event to an openapi catalogue",
		},

		{
			name: "should_fail_for_wrong_project_type",
			args: args{
				ctx: context.Background(),
				CatalogueEvent: &models.AddEventToCatalogue{
					EventID: "1234",
					Name:    "invoice.paid",
				},
				Project: &datastore.Project{UID: "project_1", Type: datastore.IncomingProject},
			},
			wantErr:    true,
			wantErrMsg: "event catalogue is only available to outgoing projects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			c := provideAddEventToCatalogueService(ctrl, tt.args.Project, tt.args.CatalogueEvent)

			if tt.dbFn != nil {
				tt.dbFn(c)
			}

			got, err := c.Run(tt.args.ctx)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tt.want.Events, got.Events)
		})
	}
}
