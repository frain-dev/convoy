package services

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/util"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideDeviceService(ctrl *gomock.Controller) *DeviceService {
	deviceRepo := mocks.NewMockDeviceRepository(ctrl)
	return NewDeviceService(deviceRepo)
}

func TestDeviceService_LoadDevicesPaged(t *testing.T) {
	ctx := context.Background()

	type args struct {
		ctx      context.Context
		group    *datastore.Group
		filter   *datastore.ApiKeyFilter
		pageable datastore.Pageable
	}

	tests := []struct {
		name               string
		args               args
		dbFn               func(d *DeviceService)
		wantDevices        []datastore.Device
		wantPaginationData datastore.PaginationData
		wantErr            bool
		wantErrCode        int
		wantErrMsg         string
	}{
		{
			name: "should_load_devices",
			args: args{
				ctx:    ctx,
				group:  &datastore.Group{UID: "12345"},
				filter: &datastore.ApiKeyFilter{EndpointID: ""},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			wantDevices: []datastore.Device{
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
			dbFn: func(d *DeviceService) {
				dr, _ := d.deviceRepo.(*mocks.MockDeviceRepository)
				dr.EXPECT().
					LoadDevicesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return([]datastore.Device{
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
			name: "should_fail_to_load_devices",
			args: args{
				ctx:   ctx,
				group: &datastore.Group{UID: "12345"},
				pageable: datastore.Pageable{
					Page:    1,
					PerPage: 10,
					Sort:    1,
				},
			},
			dbFn: func(d *DeviceService) {
				dr, _ := d.deviceRepo.(*mocks.MockDeviceRepository)
				dr.EXPECT().LoadDevicesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(1).
					Return(nil, datastore.PaginationData{}, errors.New("failed"))
			},
			wantErr:     true,
			wantErrCode: http.StatusInternalServerError,
			wantErrMsg:  "an error occurred while fetching devices",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			ds := provideDeviceService(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(ds)
			}

			devices, paginationData, err := ds.LoadDevicesPaged(tc.args.ctx, tc.args.group, tc.args.filter, tc.args.pageable)
			if tc.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tc.wantErrCode, err.(*util.ServiceError).ErrCode())
				require.Equal(t, tc.wantErrMsg, err.(*util.ServiceError).Error())
				return
			}

			require.Nil(t, err)
			require.Equal(t, tc.wantDevices, devices)
			require.Equal(t, tc.wantPaginationData, paginationData)
		})
	}
}
