package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdatePortalLinkService(ctrl *gomock.Controller, project *datastore.Project, update *models.PortalLink, link *datastore.PortalLink) *UpdatePortalLinkService {
	return &UpdatePortalLinkService{
		PortalLinkRepo: mocks.NewMockPortalLinkRepository(ctrl),
		EndpointRepo:   mocks.NewMockEndpointRepository(ctrl),
		Project:        project,
		Update:         update,
		PortalLink:     link,
	}
}

func TestUpdatePortalLinkService_Run(t *testing.T) {
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
		dbFn           func(pl *UpdatePortalLinkService)
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
			dbFn: func(pl *UpdatePortalLinkService) {
				p, _ := pl.PortalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().UpdatePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)

				e, _ := pl.EndpointRepo.(*mocks.MockEndpointRepository)
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
			dbFn: func(pl *UpdatePortalLinkService) {
				p, _ := pl.PortalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().UpdatePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				e, _ := pl.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "1234",
					ProjectID: "12345",
				}, nil)
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while updating portal link",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pl := provideUpdatePortalLinkService(ctrl, tc.args.project, tc.args.update, tc.args.portalLink)

			if tc.dbFn != nil {
				tc.dbFn(pl)
			}

			portalLink, err := pl.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, portalLink.UID, tc.wantPortalLink.UID)
			require.Equal(t, portalLink.Name, tc.wantPortalLink.Name)
			require.Equal(t, portalLink.Endpoints, tc.wantPortalLink.Endpoints)
		})
	}
}
