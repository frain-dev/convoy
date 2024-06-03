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

func provideCreatePortalLinkService(ctrl *gomock.Controller, portal *models.PortalLink, project *datastore.Project) *CreatePortalLinkService {
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
		newPortalLink *models.PortalLink
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
			dbFn: func(pl *CreatePortalLinkService) {
				p, _ := pl.PortalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().CreatePortalLink(gomock.Any(), gomock.Any()).Times(1).Return(nil)

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
			name: "should_error_for_emtpy_endpoints_and_ownerID",
			args: args{
				ctx: ctx,
				newPortalLink: &models.PortalLink{
					Name:      "test_portal_link",
					Endpoints: []string{},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantErr:    true,
			wantErrMsg: ErrInvalidEndpoints.Error(),
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
			dbFn: func(pl *CreatePortalLinkService) {
				p, _ := pl.PortalLinkRepo.(*mocks.MockPortalLinkRepository)
				p.EXPECT().CreatePortalLink(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))

				e, _ := pl.EndpointRepo.(*mocks.MockEndpointRepository)
				e.EXPECT().FindEndpointByID(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(&datastore.Endpoint{
					UID:       "123",
					ProjectID: "12345",
				}, nil)
			},
			wantErr:    true,
			wantErrMsg: "failed to create portal link",
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
