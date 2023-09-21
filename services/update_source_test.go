package services

import (
	"context"
	"errors"
	"testing"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

func provideUpdateSourceService(ctrl *gomock.Controller, t *testing.T, sourceUpdate *models.UpdateSource, source *datastore.Source, project *datastore.Project) *UpdateSourceService {
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.Nil(t, err)

	return &UpdateSourceService{
		SourceRepo:   mocks.NewMockSourceRepository(ctrl),
		Cache:        mocks.NewMockCache(ctrl),
		Project:      project,
		SourceUpdate: sourceUpdate,
		Source:       source,
	}
}

func TestUpdateSourceService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx     context.Context
		source  *datastore.Source
		update  *models.UpdateSource
		project *datastore.Project
	}

	tests := []struct {
		name       string
		args       args
		wantErr    bool
		wantSource *datastore.Source
		dbFn       func(so *UpdateSourceService)
		wantErrMsg string
	}{
		{
			name: "should_update_source",
			args: args{
				ctx: ctx,
				source: &datastore.Source{
					UID: "12345",
					CustomResponse: datastore.CustomResponse{
						Body:        "triggered",
						ContentType: "text/plain",
					},
				},
				update: &models.UpdateSource{
					Name: stringPtr("Convoy-Prod"),
					CustomResponse: models.UpdateCustomResponse{
						Body:        stringPtr("[accepted]"),
						ContentType: stringPtr("application/json"),
					},
					Type: datastore.HTTPSource,
					Verifier: models.VerifierConfig{
						Type: datastore.HMacVerifier,
						HMac: &models.HMac{
							Encoding: datastore.Base64Encoding,
							Header:   "X-Convoy-Header",
							Hash:     "SHA512",
							Secret:   "Convoy-Secret",
						},
					},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantSource: &datastore.Source{
				Name: "Convoy-Prod",
				Type: datastore.HTTPSource,
				CustomResponse: datastore.CustomResponse{
					Body:        "[accepted]",
					ContentType: "application/json",
				},
				Verifier: &datastore.VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: &datastore.HMac{
						Encoding: datastore.Base64Encoding,
						Header:   "X-Convoy-Header",
						Hash:     "SHA512",
						Secret:   "Convoy-Secret",
					},
				},
			},
			dbFn: func(so *UpdateSourceService) {
				s, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().UpdateSource(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},

		{
			name: "should_fail_to_update_source",
			args: args{
				ctx:    ctx,
				source: &datastore.Source{UID: "12345"},
				update: &models.UpdateSource{
					Name: stringPtr("Convoy-Prod"),
					Type: datastore.HTTPSource,
					Verifier: models.VerifierConfig{
						Type: datastore.HMacVerifier,
						HMac: &models.HMac{
							Encoding: datastore.Base64Encoding,
							Header:   "X-Convoy-Header",
							Hash:     "SHA512",
							Secret:   "Convoy-Secret",
						},
					},
				},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(so *UpdateSourceService) {
				s, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().UpdateSource(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("updated failed"))
			},
			wantErr:    true,
			wantErrMsg: "an error occurred while updating source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			so := provideUpdateSourceService(ctrl, t, tc.args.update, tc.args.source, tc.args.project)

			if tc.dbFn != nil {
				tc.dbFn(so)
			}

			source, err := so.Run(tc.args.ctx)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.NotEmpty(t, source.UID)

			require.Equal(t, source.Name, tc.wantSource.Name)
			require.Equal(t, source.Type, tc.wantSource.Type)
			require.Equal(t, source.Verifier.Type, tc.wantSource.Verifier.Type)
			require.Equal(t, source.Verifier.HMac.Header, tc.wantSource.Verifier.HMac.Header)
		})
	}
}
