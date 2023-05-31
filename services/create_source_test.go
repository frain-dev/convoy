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

func provideCreateSourceService(ctrl *gomock.Controller, t *testing.T, newSource *models.Source, project *datastore.Project) *CreateSourceService {
	err := config.LoadConfig("./testdata/Auth_Config/full-convoy.json")
	require.Nil(t, err)

	return &CreateSourceService{
		SourceRepo: mocks.NewMockSourceRepository(ctrl),
		Cache:      mocks.NewMockCache(ctrl),
		NewSource:  newSource,
		Project:    project,
	}
}

func TestCreateSourceService_Run(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		newSource *models.Source
		project   *datastore.Project
	}

	tests := []struct {
		name       string
		args       args
		wantSource *datastore.Source
		dbFn       func(so *CreateSourceService)
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_create_source",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name: "Convoy-Prod",
					Type: datastore.HTTPSource,
					CustomResponse: models.CustomResponse{
						Body:        "[accepted]",
						ContentType: "application/json",
					},
					Verifier: datastore.VerifierConfig{
						Type: datastore.HMacVerifier,
						HMac: &datastore.HMac{
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
						Header: "X-Convoy-Header",
						Hash:   "SHA512",
						Secret: "Convoy-Secret",
					},
				},
			},
			dbFn: func(so *CreateSourceService) {
				s, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().CreateSource(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name: "should_create_github_source",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name:     "Convoy-Prod",
					Type:     datastore.HTTPSource,
					Provider: datastore.GithubSourceProvider,
					Verifier: datastore.VerifierConfig{
						HMac: &datastore.HMac{
							Secret: "Convoy-Secret",
						},
					},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantSource: &datastore.Source{
				Name:     "Convoy-Prod",
				Type:     datastore.HTTPSource,
				Provider: datastore.GithubSourceProvider,
				Verifier: &datastore.VerifierConfig{
					HMac: &datastore.HMac{
						Secret: "Convoy-Secret",
					},
				},
			},
			dbFn: func(so *CreateSourceService) {
				s, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().CreateSource(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name: "should_error_for_empty_name",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name:     "",
					Type:     datastore.HTTPSource,
					Provider: datastore.GithubSourceProvider,
					Verifier: datastore.VerifierConfig{
						HMac: &datastore.HMac{
							Secret: "Convoy-Secret",
						},
					},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantErr:    true,
			wantErrMsg: "please provide a source name",
		},
		{
			name: "should_error_for_invalid_type",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name:     "Convoy-Prod",
					Type:     "abc",
					Provider: datastore.GithubSourceProvider,
					Verifier: datastore.VerifierConfig{
						HMac: &datastore.HMac{
							Secret: "Convoy-Secret",
						},
					},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantErr:    true,
			wantErrMsg: "please provide a valid source type",
		},
		{
			name: "should_error_for_empty_hmac_secret",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name:     "Convoy-Prod",
					Type:     datastore.HTTPSource,
					Provider: datastore.GithubSourceProvider,
					Verifier: datastore.VerifierConfig{
						HMac: &datastore.HMac{
							Secret: "",
						},
					},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantErr:    true,
			wantErrMsg: "hmac secret is required for github source",
		},
		{
			name: "should_error_for_nil_hmac",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name:     "Convoy-Prod",
					Type:     datastore.HTTPSource,
					Provider: datastore.GithubSourceProvider,
					Verifier: datastore.VerifierConfig{HMac: nil},
				},
				project: &datastore.Project{UID: "12345"},
			},
			wantErr:    true,
			wantErrMsg: "hmac secret is required for github source",
		},
		{
			name: "should_set_default_forward_header_for_shopify_source",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name:     "Convoy-Prod",
					Type:     datastore.HTTPSource,
					Provider: datastore.ShopifySourceProvider,
					Verifier: datastore.VerifierConfig{
						Type: datastore.HMacVerifier,
						HMac: &datastore.HMac{
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
				ForwardHeaders: []string{
					"X-Shopify-Topic",
					"X-Shopify-Hmac-Sha256",
					"X-Shopify-Shop-Domain",
					"X-Shopify-API-Version",
					"X-Shopify-Webhook-Id",
				},
				Verifier: &datastore.VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: &datastore.HMac{
						Header: "X-Convoy-Header",
						Hash:   "SHA512",
						Secret: "Convoy-Secret",
					},
				},
			},
			dbFn: func(so *CreateSourceService) {
				s, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().CreateSource(gomock.Any(), gomock.Any()).Times(1).Return(nil)
			},
		},
		{
			name: "should_fail_to_create_source",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name: "Convoy-Prod",
					Type: datastore.HTTPSource,
					Verifier: datastore.VerifierConfig{
						Type: datastore.HMacVerifier,
						HMac: &datastore.HMac{
							Encoding: datastore.Base64Encoding,
							Header:   "X-Convoy-Header",
							Hash:     "SHA512",
							Secret:   "Convoy-Secret",
						},
					},
				},
				project: &datastore.Project{
					UID: "12345",
				},
			},
			dbFn: func(so *CreateSourceService) {
				s, _ := so.SourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().CreateSource(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:    true,
			wantErrMsg: "failed to create source",
		},
		{
			name: "should_fail_invalid_source_configuration",
			args: args{
				ctx: ctx,
				newSource: &models.Source{
					Name: "Convoy-Prod",
					Type: datastore.HTTPSource,
					Verifier: datastore.VerifierConfig{
						Type: datastore.HMacVerifier,
					},
				},
				project: &datastore.Project{
					UID: "12345",
				},
			},
			dbFn:       func(so *CreateSourceService) {},
			wantErr:    true,
			wantErrMsg: "Invalid verifier config for hmac",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			so := provideCreateSourceService(ctrl, t, tc.args.newSource, tc.args.project)

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
			require.NotEmpty(t, source.MaskID)

			require.Equal(t, source.Name, tc.wantSource.Name)
			require.Equal(t, source.Type, tc.wantSource.Type)
			require.Equal(t, source.Verifier.Type, tc.wantSource.Verifier.Type)
			require.Equal(t, source.Verifier.HMac.Header, tc.wantSource.Verifier.HMac.Header)
		})
	}
}
