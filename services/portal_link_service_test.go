package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func providePortalLinkService(ctrl *gomock.Controller) *PortalLinkService {
	portalRepo := mocks.NewMockPortalLinkRepository(ctrl)
	endpointSerivce := provideEndpointService(ctrl)
	return NewPortalLinkService(portalRepo, endpointSerivce)
}

func TestPortalLinkService_CreatePortalLinK(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx           context.Context
		newPortalLink *models.PortalLink
		group         *datastore.Group
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
				group: &datastore.Group{UID: "12345"},
			},
			wantPortalLink: &datastore.PortalLink{
				GroupID:        "12345",
				Endpoints:      []string{"123", "1234"},
				DocumentStatus: datastore.ActiveDocumentStatus,
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().CreatePortalLink(gomock.Any(), gomock.Any()).Times(1).Return(nil)

				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:     "123",
					GroupID: "12345",
				}, nil)

				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:     "1234",
					GroupID: "12345",
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
				group: &datastore.Group{UID: "12345"},
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
				group: &datastore.Group{UID: "12345"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().CreatePortalLink(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:     "123",
					GroupID: "12345",
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

			portalLink, err := pl.CreatePortalLink(tc.args.ctx, tc.args.newPortalLink, tc.args.group)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, portalLink.UID)

			require.Equal(t, portalLink.GroupID, tc.wantPortalLink.GroupID)
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
		group      *datastore.Group
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
				group: &datastore.Group{UID: "12345"},
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
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:     "123",
					GroupID: "12345",
				}, nil)

				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:     "1234",
					GroupID: "12345",
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
				group: &datastore.Group{UID: "12345"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().UpdatePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:     "1234",
					GroupID: "12345",
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

			portalLink, err := pl.UpdatePortalLink(tc.args.ctx, tc.args.group, tc.args.update, tc.args.portalLink)
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

func TestPortalLinkService_FindPortalLinkByID(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx   context.Context
		group *datastore.Group
		uid   string
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
			name: "should_find_portal_link_by_id",
			args: args{
				ctx:   ctx,
				uid:   "1234",
				group: &datastore.Group{UID: "12345"},
			},
			wantPortalLink: &datastore.PortalLink{
				UID:       "1234",
				Name:      "test_portal_link",
				Endpoints: []string{"123"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().FindPortalLinkByID(gomock.Any(), gomock.Any(), gomock.Any()).
					Times(1).Return(&datastore.PortalLink{
					UID:       "1234",
					Name:      "test_portal_link",
					Endpoints: []string{"123"},
				}, nil)
			},
		},

		{
			name: "should_fail_to_find_portal_link_by_id",
			args: args{
				ctx:   ctx,
				uid:   "1234",
				group: &datastore.Group{UID: "12345"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().FindPortalLinkByID(gomock.Any(), gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "error retrieving portal link",
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

			portalLink, err := pl.FindPortalLinkByID(tc.args.ctx, tc.args.group, tc.args.uid)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, portalLink.UID, tc.wantPortalLink.UID)
			require.Equal(t, portalLink.Endpoints, tc.wantPortalLink.Endpoints)
			require.Equal(t, portalLink.Name, tc.wantPortalLink.Name)
		})
	}
}

func TestPortalLinkService_RevokePortalLink(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx        context.Context
		portalLink *datastore.PortalLink
		group      *datastore.Group
	}

	tests := []struct {
		name        string
		args        args
		dbFn        func(pl *PortalLinkService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_revoke_portal_link",
			args: args{
				ctx: ctx,
				portalLink: &datastore.PortalLink{
					UID:       "12345",
					Endpoints: []string{"123"},
				},
				group: &datastore.Group{UID: "12345"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().RevokePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},

		{
			name: "should_fail_to_revoke_portal_link",
			args: args{
				ctx: ctx,
				portalLink: &datastore.PortalLink{
					UID:       "12345",
					Endpoints: []string{"123"},
				},
				group: &datastore.Group{UID: "12345"},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().RevokePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete portal link",
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

			err := pl.RevokePortalLink(tc.args.ctx, tc.args.group, tc.args.portalLink)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
		})
	}
}

func TestPortalLinkService_LoadPortalLinksPaged(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx      context.Context
		group    *datastore.Group
		pageable datastore.Pageable
	}

	tests := []struct {
		name               string
		args               args
		dbFn               func(pl *PortalLinkService)
		wantPortalLinks    []datastore.PortalLink
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_portal_links",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			wantPortalLinks: []datastore.PortalLink{
				{UID: "12345"},
				{UID: "123456"},
			},
			wantPaginationData: datastore.PaginationData{
				Total:     2,
				Page:      1,
				PerPage:   10,
				Prev:      0,
				Next:      2,
				TotalPage: 3,
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().LoadPortalLinksPaged(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.PortalLink{
						{UID: "12345"},
						{UID: "123456"},
					}, datastore.PaginationData{
						Total:     2,
						Page:      1,
						PerPage:   10,
						Prev:      0,
						Next:      2,
						TotalPage: 3,
					}, nil)
			},
		},

		{
			name: "should_fail_to_load_portal_links",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			dbFn: func(pl *PortalLinkService) {
				p, _ := pl.portalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().LoadPortalLinksPaged(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching portal links",
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

			portalLinks, paginationData, err := pl.LoadPortalLinksPaged(tc.args.ctx, tc.args.group, tc.args.pageable)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.wantPortalLinks, portalLinks)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}

func TestPortalLinkService_GetPortalLinkEndpoints(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx        context.Context
		portalLink *datastore.PortalLink
		group      *datastore.Group
	}

	tests := []struct {
		name          string
		args          args
		wantEndpoints []datastore.Endpoint
		dbFn          func(pl *PortalLinkService)
		wantErr       bool
		wantErrCode   int
		wantErrMsg    string
	}{
		{
			name: "should_get_portal_link_endpoints",
			args: args{
				ctx: ctx,
				portalLink: &datastore.PortalLink{
					UID:       "123",
					Endpoints: []string{"123", "1234"},
				},
				group: &datastore.Group{
					UID: "12345",
				},
			},
			dbFn: func(pl *PortalLinkService) {
				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)

				e.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Endpoint{
						{UID: "123"},
						{UID: "1234"},
					}, nil)
			},
			wantEndpoints: []datastore.Endpoint{
				{UID: "123"},
				{UID: "1234"},
			},
		},

		{
			name: "should_fail_to_get_portal_link_endpoints",
			args: args{
				ctx: ctx,
				portalLink: &datastore.PortalLink{
					UID:       "123",
					Endpoints: []string{"123", "1234"},
				},
				group: &datastore.Group{
					UID: "12345",
				},
			},
			dbFn: func(pl *PortalLinkService) {
				e, _ := pl.endpointRepo.(*mocks.MockEndpointRepository)

				e.EXPECT().FindEndpointsByID(gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Endpoint{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching endpoints",
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

			endpoints, err := pl.GetPortalLinkEndpoints(tc.args.ctx, tc.args.portalLink)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantEndpoints, endpoints)
		})
	}
}
