package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
)

func provideUpdatePortalLinkService(ctrl *gomock.Controller, project *datastore.Project, update *models.UpdatePortalLinkRequest, link *datastore.PortalLink) *UpdatePortalLinkService {
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
		update     *models.UpdatePortalLinkRequest
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
					UID:     "12345",
					OwnerID: "1234",
				},
				update: &models.UpdatePortalLinkRequest{
					Name:     "test_portal_link",
					OwnerID:  "12345",
					AuthType: string(datastore.PortalAuthTypeRefreshToken),
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantPortalLink: &datastore.PortalLink{
				UID:      "12345",
				Name:     "test_portal_link",
				OwnerID:  "12345",
				AuthType: datastore.PortalAuthTypeRefreshToken,
			},
			dbFn: func(pl *UpdatePortalLinkService) {
				p, _ := pl.PortalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().UpdatePortalLink(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},

		{
			name: "should_fail_to_update_portal_link",
			args: args{
				ctx: ctx,
				portalLink: &datastore.PortalLink{
					UID:     "12345",
					OwnerID: "1234",
				},
				update: &models.UpdatePortalLinkRequest{
					Name:     "test_portal_link",
					AuthType: "foo",
				},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn:       func(pl *UpdatePortalLinkService) {},
			wantErr:    true,
			wantErrMsg: "invalid auth type: foo",
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
