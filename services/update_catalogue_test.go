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

func provideUpdateCatalogueService(ctrl *gomock.Controller, p *datastore.Project, e *models.UpdateCatalogue) *UpdateCatalogueService {
	return &UpdateCatalogueService{
		CatalogueRepo:   mocks.NewMockEventCatalogueRepository(ctrl),
		UpdateCatalogue: e,
		Project:         p,
	}
}

func TestUpdateCatalogueService_Run(t *testing.T) {
	type args struct {
		UpdateCatalogue *models.UpdateCatalogue
		Project         *datastore.Project
		ctx             context.Context
	}
	tests := []struct {
		name       string
		dbFn       func(es *UpdateCatalogueService)
		args       args
		want       *datastore.EventCatalogue
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_update_openapi_catalogue",
			dbFn: func(es *UpdateCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(
					&datastore.EventCatalogue{
						ProjectID:   "project_1",
						Type:        datastore.OpenAPICatalogueType,
						OpenAPISpec: []byte(`spec`),
					},
					nil,
				)

				cr.EXPECT().UpdateEventCatalogue(gomock.Any(), &datastore.EventCatalogue{
					ProjectID:   "project_1",
					Type:        datastore.OpenAPICatalogueType,
					OpenAPISpec: []byte(`yaml`),
				}).Times(1).Return(nil)
			},
			args: args{
				UpdateCatalogue: &models.UpdateCatalogue{
					Events:      nil,
					OpenAPISpec: []byte(`yaml`),
				},
				Project: &datastore.Project{UID: "project_1"},
				ctx:     context.Background(),
			},
			want: &datastore.EventCatalogue{
				ProjectID:   "project_1",
				Type:        datastore.OpenAPICatalogueType,
				OpenAPISpec: []byte(`yaml`),
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_update_events_catalogue",
			dbFn: func(es *UpdateCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(
					&datastore.EventCatalogue{
						ProjectID: "project_1",
						Type:      datastore.EventsDataCatalogueType,
						Events: datastore.EventDataCatalogues{
							{
								EventID: "1234",
								Name:    "invoice.paid",
								Data:    []byte(`{"name":"daniel"}`),
							},
						},
					},
					nil,
				)

				cr.EXPECT().UpdateEventCatalogue(gomock.Any(), &datastore.EventCatalogue{
					ProjectID: "project_1",
					Type:      datastore.EventsDataCatalogueType,
					Events: datastore.EventDataCatalogues{
						{
							EventID: "1234",
							Name:    "invoice.paid",
							Data:    []byte(`{"name":"daniel"}`),
						},
						{
							EventID: "abc",
							Name:    "invoice.created",
							Data:    []byte(`{"name":"raymond"}`),
						},
					},
				}).Times(1).Return(nil)
			},
			args: args{
				UpdateCatalogue: &models.UpdateCatalogue{
					Events: datastore.EventDataCatalogues{
						{
							EventID: "1234",
							Name:    "invoice.paid",
							Data:    []byte(`{"name":"daniel"}`),
						},
						{
							EventID: "abc",
							Name:    "invoice.created",
							Data:    []byte(`{"name":"raymond"}`),
						},
					},
				},
				Project: &datastore.Project{UID: "project_1"},
				ctx:     context.Background(),
			},
			want: &datastore.EventCatalogue{
				ProjectID: "project_1",
				Type:      datastore.EventsDataCatalogueType,
				Events: datastore.EventDataCatalogues{
					{
						EventID: "1234",
						Name:    "invoice.paid",
						Data:    []byte(`{"name":"daniel"}`),
					},
					{
						EventID: "abc",
						Name:    "invoice.created",
						Data:    []byte(`{"name":"raymond"}`),
					},
				},
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_fail_to_fetch_catalogue",
			dbFn: func(es *UpdateCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(
					nil,
					errors.New("failed"),
				)
			},
			args: args{
				UpdateCatalogue: &models.UpdateCatalogue{
					Events: datastore.EventDataCatalogues{
						{
							EventID: "abc",
							Name:    "invoice.created",
							Data:    []byte(`{"name":"raymond"}`),
						},
					},
				},
				Project: &datastore.Project{UID: "project_1"},
				ctx:     context.Background(),
			},
			wantErr:    true,
			wantErrMsg: "unable to fetch catalogue",
		},
		{
			name: "should_fail_to_update_catalogue",
			dbFn: func(es *UpdateCatalogueService) {
				cr, _ := es.CatalogueRepo.(*mocks.MockEventCatalogueRepository)
				cr.EXPECT().FindEventCatalogueByProjectID(gomock.Any(), "project_1").Times(1).Return(
					&datastore.EventCatalogue{
						ProjectID: "project_1",
						Type:      datastore.EventsDataCatalogueType,
						Events: datastore.EventDataCatalogues{
							{
								EventID: "1234",
								Name:    "invoice.paid",
								Data:    []byte(`{"name":"daniel"}`),
							},
						},
					},
					nil,
				)

				cr.EXPECT().UpdateEventCatalogue(gomock.Any(), &datastore.EventCatalogue{
					ProjectID: "project_1",
					Type:      datastore.EventsDataCatalogueType,
					Events: datastore.EventDataCatalogues{
						{
							EventID: "1234",
							Name:    "invoice.paid",
							Data:    []byte(`{"name":"daniel"}`),
						},
						{
							EventID: "abc",
							Name:    "invoice.created",
							Data:    []byte(`{"name":"raymond"}`),
						},
					},
				}).Times(1).Return(errors.New("failed"))
			},
			args: args{
				UpdateCatalogue: &models.UpdateCatalogue{
					Events: datastore.EventDataCatalogues{
						{
							EventID: "1234",
							Name:    "invoice.paid",
							Data:    []byte(`{"name":"daniel"}`),
						},
						{
							EventID: "abc",
							Name:    "invoice.created",
							Data:    []byte(`{"name":"raymond"}`),
						},
					},
				},
				Project: &datastore.Project{UID: "project_1"},
				ctx:     context.Background(),
			},
			wantErr:    true,
			wantErrMsg: "failed to update catalogue",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			err := config.LoadConfig("./testdata/basic-config.json")
			require.NoError(t, err)

			c := provideUpdateCatalogueService(ctrl, tt.args.Project, tt.args.UpdateCatalogue)

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
			require.Equal(t, tt.want, got)
		})
	}
}
