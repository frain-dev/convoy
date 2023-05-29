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

func provideSourceService(ctrl *gomock.Controller) *SourceService {
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	cache := mocks.NewMockCache(ctrl)
	return NewSourceService(sourceRepo, cache)
}

func TestSourceService_CreateSource(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		newSource *models.Source
		project   *datastore.Project
	}

	tests := []struct {
		name        string
		args        args
		wantSource  *datastore.Source
		dbFn        func(so *SourceService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
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
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
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
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
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
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "please provide a source name",
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
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "please provide a valid source type",
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
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "hmac secret is required for github source",
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
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "hmac secret is required for github source",
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
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
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
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().CreateSource(gomock.Any(), gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to create source",
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
			dbFn:        func(so *SourceService) {},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "Invalid verifier config for hmac",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			so := provideSourceService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(so)
			}

			source, err := so.CreateSource(tc.args.ctx, tc.args.newSource, tc.args.project)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
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

func TestSourceService_UpdateSource(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx     context.Context
		source  *datastore.Source
		update  *models.UpdateSource
		project *datastore.Project
	}

	tests := []struct {
		name        string
		args        args
		wantErr     bool
		wantSource  *datastore.Source
		dbFn        func(so *SourceService)
		wantErrCode int
		wantErrMsg  string
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
						Encoding: datastore.Base64Encoding,
						Header:   "X-Convoy-Header",
						Hash:     "SHA512",
						Secret:   "Convoy-Secret",
					},
				},
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
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
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().UpdateSource(gomock.Any(), gomock.Any(), gomock.Any()).Times(1).Return(errors.New("updated failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "an error occurred while updating source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			so := provideSourceService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(so)
			}

			source, err := so.UpdateSource(tc.args.ctx, tc.args.project, tc.args.update, tc.args.source)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
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
