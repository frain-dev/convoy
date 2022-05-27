package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/server/models"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSourceService(ctrl *gomock.Controller) *SourceService {
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	return NewSourceService(sourceRepo)
}

func TestSourceService_CreateSource(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx       context.Context
		newSource *models.Source
		group     *datastore.Group
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
						HMac: datastore.HMac{
							Header: "X-Convoy-Header",
							Hash:   "SHA512",
							Secret: "Convoy-Secret",
						},
					},
				},
				group: &datastore.Group{UID: "12345"},
			},
			wantSource: &datastore.Source{
				Name: "Convoy-Prod",
				Type: datastore.HTTPSource,
				Verifier: &datastore.VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: datastore.HMac{
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
						HMac: datastore.HMac{
							Header: "X-Convoy-Header",
							Hash:   "SHA512",
							Secret: "Convoy-Secret",
						},
					},
				},
				group: &datastore.Group{
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			so := provideSourceService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(so)
			}

			source, err := so.CreateSource(tc.args.ctx, tc.args.newSource, tc.args.group)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
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

func TestSourceService_UpdateSource(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx    context.Context
		source *datastore.Source
		update *models.UpdateSource
		group  *datastore.Group
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
						HMac: datastore.HMac{
							Header: "X-Convoy-Header",
							Hash:   "SHA512",
							Secret: "Convoy-Secret",
						},
					},
				},
				group: &datastore.Group{UID: "12345"},
			},
			wantSource: &datastore.Source{
				Name: "Convoy-Prod",
				Type: datastore.HTTPSource,
				Verifier: &datastore.VerifierConfig{
					Type: datastore.HMacVerifier,
					HMac: datastore.HMac{
						Header: "X-Convoy-Header",
						Hash:   "SHA512",
						Secret: "Convoy-Secret",
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
						HMac: datastore.HMac{
							Header: "X-Convoy-Header",
							Hash:   "SHA512",
							Secret: "Convoy-Secret",
						},
					},
				},
				group: &datastore.Group{UID: "12345"},
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

			source, err := so.UpdateSource(tc.args.ctx, tc.args.group, tc.args.update, tc.args.source)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
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

func TestSourceService_FindSourceByID(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx   context.Context
		uid   string
		group *datastore.Group
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
				ctx:   ctx,
				uid:   "1234",
				group: &datastore.Group{UID: "12345"},
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
				ctx:   ctx,
				uid:   "1234",
				group: &datastore.Group{UID: "12345"},
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

			source, err := so.FindSourceByID(tc.args.ctx, tc.args.group, tc.args.uid)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.wantSource, source)
		})
	}
}

// func TestSourceService_FindSourceByMaskID(t *testing.T) {
// 	ctx := context.Background()

// 	type args struct {
// 		ctx    context.Context
// 		maskID string
// 		group  *datastore.Group
// 	}

// 	tests := []struct {
// 		name        string
// 		args        args
// 		wantSource  *datastore.Source
// 		dbFn        func(so *SourceService)
// 		wantErr     bool
// 		wantErrCode int
// 		wantErrMsg  string
// 	}{
// 		{
// 			name: "should_find_source_by_id",
// 			args: args{
// 				ctx:    ctx,
// 				maskID: "1234",
// 				group:  &datastore.Group{UID: "12345"},
// 			},
// 			wantSource: &datastore.Source{MaskID: "1234"},
// 			dbFn: func(so *SourceService) {
// 				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
// 				s.EXPECT().FindSourceByMaskID(gomock.Any(), "1234").Times(1).Return(&datastore.Source{MaskID: "1234"}, nil)
// 			},
// 		},

// 		{
// 			name: "should_fail_to_find_source_by_id",
// 			args: args{
// 				ctx:    ctx,
// 				maskID: "1234",
// 				group:  &datastore.Group{UID: "12345"},
// 			},
// 			dbFn: func(so *SourceService) {
// 				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
// 				s.EXPECT().FindSourceByMaskID(gomock.Any(), "1234").Times(1).Return(nil, errors.New("failed"))
// 			},
// 			wantErr:     true,
// 			wantErrCode: http.StatusBadRequest,
// 			wantErrMsg:  "error retrieving source",
// 		},
// 	}

// 	for _, tc := range tests {
// 		t.Run(tc.name, func(t *testing.T) {
// 			ctrl := gomock.NewController(t)
// 			defer ctrl.Finish()

// 			so := provideSourceService(ctrl)

// 			if tc.dbFn != nil {
// 				tc.dbFn(so)
// 			}

// 			source, err := so.FindSourceByMaskID(tc.args.ctx, tc.args.group, tc.args.maskID)
// 			if tc.wantErr {
// 				require.NotNil(t, err)
// 				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
// 				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
// 				return
// 			}
// 			require.Nil(t, err)
// 			require.Equal(t, tc.wantSource, source)
// 		})
// 	}
// }

func TestSourceService_DeleteSourceByID(t *testing.T) {
	ctx := context.Background()
	type args struct {
		ctx   context.Context
		id    string
		group *datastore.Group
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
				ctx:   ctx,
				id:    "12345",
				group: &datastore.Group{UID: "12345"},
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().DeleteSourceByID(gomock.Any(), gomock.Any(), "12345").Times(1).Return(nil)
			},
		},

		{
			name: "should_fail_to_delete_source",
			args: args{
				ctx:   ctx,
				id:    "12345",
				group: &datastore.Group{UID: "12345"},
			},
			dbFn: func(so *SourceService) {
				s, _ := so.sourceRepo.(*mocks.MockSourceRepository)
				s.EXPECT().DeleteSourceByID(gomock.Any(), gomock.Any(), "12345").Times(1).Return(errors.New("failed"))
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

			err := so.DeleteSourceByID(tc.args.ctx, tc.args.group, tc.args.id)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
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
		group    *datastore.Group
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
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
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
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
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

			sources, paginationData, err := so.LoadSourcesPaged(tc.args.ctx, tc.args.group, tc.args.filter, tc.args.pageable)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*ServiceError).Error())
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.wantSources, sources)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}
