package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func providePortalLinkService(ctrl *gomock.Controller) *PortalLinkService {
	portalRepo := mocks.NewMockPortalLinkRepository(ctrl)
	projectRepo := mocks.NewMockProjectRepository(ctrl)
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)

	return NewPortalLinkService(portalRepo, endpointRepo, cache, projectRepo)
}

func TestPortalLinkService_CreatePortalLinK(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx           context.Context
		newPortalLink *models.PortalLink
		project       *datastore.Project
	}

	tests := []struct {
		name           string
		args           args
		wantPortalLink *datastore.PortalLink
		dbFn           func(pl *PortalLinkService)
		wantErr        bool
		wantErrCode    int
		wantErrMsg     string
	}{
		{
			name: "should_create_portal_link",
			args: args{
				ctx: ctx,
				newPortalLink: &models.PortalLink{
					Name:      "test_portal_link",
					Endpoints: []string{"123", "1234"},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantPortalLink: &datastore.PortalLink{
				ProjectID: "12345",
				Endpoints: []string{"123", "1234"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().CreatePortalLink(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "123",
					ProjectID: "12345",
				}, nil)

				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "1234",
					ProjectID: "12345",
				}, nil)
			},
		},

		{
			name: "should_error_for_emtpy_endpoints",
			args: args{
				ctx: ctx,
				newPortalLink: &models.PortalLink{
					Name:      "test_portal_link",
					Endpoints: []string{},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  ErrInvalidEndpoints.Error(),
		},

		{
			name: "should_fail_to_create_portal_link",
			args: args{
				ctx: ctx,
				newPortalLink: &models.PortalLink{
					Name:      "test_portal_link",
					Endpoints: []string{"123"},
				},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().CreatePortalLink(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "123",
					ProjectID: "12345",
				}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create portal link",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pl := providePortalLinkService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(pl)
			}

			portalLink, err := pl.CreatePortalLink(tc.args.ctx, tc.args.newPortalLink, tc.args.project)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, portalLink.UID)

			require.Equal(t, portalLink.ProjectID, tc.wantPortalLink.ProjectID)
			require.Equal(t, portalLink.Endpoints, tc.wantPortalLink.Endpoints)
		})
	}
}

func TestPortalLinkService_UpdatePortalLink(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx        context.Context
		portalLink *datastore.PortalLink
		update     *models.PortalLink
		project    *datastore.Project
	}

	tests := []struct {
		name           string
		args           args
		wantErr        bool
		wantPortalLink *datastore.PortalLink
		dbFn           func(pl *PortalLinkService)
		wantErrCode    int
		wantErrMsg     string
	}{
		{
			name: "should_update_portal_link",
			args: args{
				ctx: ctx,
				portalLink: &datastore.PortalLink{
					UID:       "12345",
					Endpoints: []string{"123"},
				},
				update: &models.PortalLink{
					Name:      "test_portal_link",
					Endpoints: []string{"123", "1234"},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantPortalLink: &datastore.PortalLink{
				UID:       "12345",
				Name:      "test_portal_link",
				Endpoints: []string{"123", "1234"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().UpdatePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "123",
					ProjectID: "12345",
				}, nil)

				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "1234",
					ProjectID: "12345",
				}, nil)
			},
		},

		{
			name: "should_fail_to_update_portal_link",
			args: args{
				ctx: ctx,
				portalLink: &datastore.PortalLink{
					UID:       "12345",
					Endpoints: []string{"123"},
				},
				update: &models.PortalLink{
					Name:      "test_portal_link",
					Endpoints: []string{"1234"},
				},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().UpdatePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "1234",
					ProjectID: "12345",
				}, nil)
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while updating portal link",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pl := providePortalLinkService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(pl)
			}

			portalLink, err := pl.UpdatePortalLink(tc.args.ctx, tc.args.project, tc.args.update, tc.args.portalLink)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, portalLink.UID, tc.wantPortalLink.UID)
			require.Equal(t, portalLink.Name, tc.wantPortalLink.Name)
			require.Equal(t, portalLink.Endpoints, tc.wantPortalLink.Endpoints)
		})
	}
}
