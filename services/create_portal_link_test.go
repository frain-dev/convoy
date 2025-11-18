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

func provideCreatePortalLinkService(ctrl *gomock.Controller, portal *models.CreatePortalLinkRequest, project *datastore.Project) *CreatePortalLinkService {
	return &CreatePortalLinkService{
		PortalLinkRepo: mocks.NewMockPortalLinkRepository(ctrl),
		EndpointRepo:   mocks.NewMockEndpointRepository(ctrl),
		Portal:         portal,
		Project:        project,
	}
}

func TestCreatePortalLinkService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx           context.Context
		newPortalLink *models.CreatePortalLinkRequest
		project       *datastore.Project
	}

	tests := []struct {
		name           string
		args           args
		wantPortalLink *datastore.PortalLink
		dbFn           func(pl *CreatePortalLinkService)
		wantErr        bool
		wantErrMsg     string
	}{
		{
			name: "should_create_portal_link",
			args: args{
				ctx: ctx,
				newPortalLink: &models.CreatePortalLinkRequest{
					Name:     "test_portal_link",
					OwnerID:  "1234",
					AuthType: "static_token",
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantPortalLink: &datastore.PortalLink{
				ProjectID: "12345",
				Name:      "test_portal_link",
				OwnerID:   "1234",
			},
			dbFn: func(pl *CreatePortalLinkService) {
				p, _ := pl.PortalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().CreatePortalLink(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},

		{
			name: "should_error_for_empty_ownerID",
			args: args{
				ctx: ctx,
				newPortalLink: &models.CreatePortalLinkRequest{
					Name: "test_portal_link",
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantErr:    true,
			wantErrMsg: "owner_id:please provide the owner id field",
		},

		{
			name: "should_error_for_empty_auth_type",
			args: args{
				ctx: ctx,
				newPortalLink: &models.CreatePortalLinkRequest{
					Name:    "test_portal_link",
					OwnerID: "foo",
				},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn:       func(pl *CreatePortalLinkService) {},
			wantErr:    true,
			wantErrMsg: "invalid auth type: ",
		},

		{
			name: "should_error_for_invalid_auth_type",
			args: args{
				ctx: ctx,
				newPortalLink: &models.CreatePortalLinkRequest{
					Name:     "test_portal_link",
					OwnerID:  "foo",
					AuthType: "foobar",
				},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn:       func(pl *CreatePortalLinkService) {},
			wantErr:    true,
			wantErrMsg: "invalid auth type: foobar",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			pl := provideCreatePortalLinkService(ctrl, tc.args.newPortalLink, tc.args.project)

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
			require.NotEmpty(t, portalLink.UID)

			require.Equal(t, portalLink.ProjectID, tc.wantPortalLink.ProjectID)
			require.Equal(t, portalLink.Endpoints, tc.wantPortalLink.Endpoints)
		})
	}
}
