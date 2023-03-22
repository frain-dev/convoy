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
			wantSource: &datastore.Source{
				Name: "Convoy-Prod",
				Type: datastore.HTTPSource,
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

func TestSourceService_FindSourceByID(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx     context.Context
		uid     string
		project *datastore.Project
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
			name: "should_find_source_by_id",
			args: args{
				ctx:     ctx,
				uid:     "1234",
				project: &datastore.Project{UID: "12345"},
			},
			wantSource: &datastore.Source{UID: "1234"},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "1234").Times(1).Return(&datastore.Source{UID: "1234"}, nil)
			},
		},

		{
			name: "should_fail_to_find_source_by_id",
			args: args{
				ctx:     ctx,
				uid:     "1234",
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().FindSourceByID(gomock.Any(), gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "error retrieving source",
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

			source, err := so.FindSourceByID(tc.args.ctx, tc.args.project, tc.args.uid)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.wantSource, source)
		})
	}
}

func TestSourceService_DeleteSource(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx     context.Context
		source  *datastore.Source
		project *datastore.Project
	}

	tests := []struct {
		name        string
		args        args
		dbFn        func(so *SourceService)
		wantErr     bool
		wantErrCode int
		wantErrMsg  string
	}{
		{
			name: "should_delete_source",
			args: args{
				ctx:     ctx,
				source:  &datastore.Source{UID: "12345", Provider: ""},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().DeleteSourceByID(gomock.Any(), gomock.Any(), "12345", gomock.Any()).Times(1).Return(nil)
			},
		},

		{
			name: "should_delete_twitter_custom_source_from_cache",
			args: args{
				ctx:     ctx,
				source:  &datastore.Source{UID: "12345", MaskID: "abcd", Provider: datastore.TwitterSourceProvider},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().DeleteSourceByID(gomock.Any(), gomock.Any(), "12345", gomock.Any()).Times(1).Return(nil)

				c, _ := so.cache.(*mocks.MockCache)
				c.EXPECT().Delete(gomock.Any(), gomock.Any())
			},
		},

		{
			name: "should_fail_to_delete_source",
			args: args{
				ctx:     ctx,
				source:  &datastore.Source{UID: "12345", Provider: ""},
				project: &datastore.Project{UID: "12345"},
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().DeleteSourceByID(gomock.Any(), gomock.Any(), "12345", gomock.Any()).Times(1).Return(errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusBadRequest,
			wantErrMsg:  "failed to delete source",
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

			err := so.DeleteSource(tc.args.ctx, tc.args.project, tc.args.source)
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

func TestSourceService_LoadSourcesPaged(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx      context.Context
		project  *datastore.Project
		pageable datastore.Pageable
		filter   *datastore.SourceFilter
	}

	tests := []struct {
		name               string
		args               args
		dbFn               func(so *SourceService)
		wantSources        []datastore.Source
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_sources",
			args: args{
				ctx:     ctx,
				project: &datastore.Project{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
				filter: nil,
			},
			wantSources: []datastore.Source{
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
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().
					LoadSourcesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Source{
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
			name: "should_fail_load_sources",
			args: args{
				ctx:     ctx,
				project: &datastore.Project{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
				filter: nil,
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().
					LoadSourcesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching sources",
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

			sources, paginationData, err := so.LoadSourcesPaged(tc.args.ctx, tc.args.project, tc.args.filter, tc.args.pageable)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.wantSources, sources)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}
