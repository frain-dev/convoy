package services

import (
	"context"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

type DeviceService struct {
	deviceRepo datastore.DeviceRepository
}

func NewDeviceService(deviceRepo datastore.DeviceRepository) *DeviceService {
	return &DeviceService{deviceRepo: deviceRepo}
}

func (d *DeviceService) LoadDevicesPaged(ctx context.Context, g *datastore.Project, f *datastore.ApiKeyFilter, pageable datastore.Pageable) ([]datastore.Device, datastore.PaginationData, error) {
	devices, paginationData, err := d.deviceRepo.LoadDevicesPaged(ctx, g.UID, f, pageable)
	if err != nil {
		return nil, datastore.PaginationData{}, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while fetching devices"))
	}

	return devices, paginationData, nil
}
