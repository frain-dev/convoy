package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/queue"
	"github.com/stretchr/testify/require"
)

type fakeAppRepo struct {
}

func (*fakeAppRepo) CreateApplication(context.Context, *datastore.Application) error {
	return nil
}

func (*fakeAppRepo) LoadApplicationsPaged(c context.Context, uid string, q string, p datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	if strings.EqualFold(q, "falsetto") {
		return []datastore.Application{}, datastore.PaginationData{}, nil
	}

	return nil, datastore.PaginationData{}, errors.New("did not trim query")
}

func (*fakeAppRepo) FindApplicationByID(context.Context, string) (*datastore.Application, error) {
	return nil, nil
}

func (*fakeAppRepo) UpdateApplication(context.Context, *datastore.Application) error {
	return nil
}

func (*fakeAppRepo) DeleteApplication(context.Context, *datastore.Application) error {
	return nil
}
func (*fakeAppRepo) CountGroupApplications(ctx context.Context, groupID string) (int64, error) {
	return 0, nil
}

func (*fakeAppRepo) DeleteGroupApps(context.Context, string) error {
	return nil
}
func (*fakeAppRepo) LoadApplicationsPagedByGroupId(context.Context, string, datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}
func (*fakeAppRepo) SearchApplicationsByGroupId(context.Context, string, datastore.SearchParams) ([]datastore.Application, error) {
	return nil, nil
}
func (*fakeAppRepo) FindApplicationEndpointByID(context.Context, string, string) (*datastore.Endpoint, error) {
	return nil, nil
}
func (*fakeAppRepo) UpdateApplicationEndpointsStatus(context.Context, string, []string, datastore.EndpointStatus) error {
	return nil
}

func Test_LoadApplicationsPaged(t *testing.T) {
	tts := []struct {
		Name              string
		AppRepo           datastore.ApplicationRepository
		EventRepo         datastore.EventRepository
		EventDeliveryRepo datastore.EventDeliveryRepository
		EventQueue        queue.Queuer
		WantErr           bool
		Uid               string
		Query             string
		Pageable          datastore.Pageable
	}{
		{
			Name:  "trims-whitespaces-from-query",
			Uid:   "uid",
			Query: " falsetto ",
			Pageable: datastore.Pageable{
				PerPage: 10,
				Page:    1,
			},
			AppRepo: &fakeAppRepo{},
		},
		{
			Name:  "trims-whitespaces-from-query-retains-value-if-no-whitespace",
			Uid:   "uid",
			Query: "falsetto",
			Pageable: datastore.Pageable{
				PerPage: 10,
				Page:    1,
			},
			AppRepo: &fakeAppRepo{},
		},
		{
			Name:  "trims-whitespaces-from-query-retains-case",
			Uid:   "uid",
			Query: " FalSetto ",
			Pageable: datastore.Pageable{
				PerPage: 10,
				Page:    1,
			},
			AppRepo: &fakeAppRepo{},
		},
	}

	for _, tt := range tts {
		var a AppService = AppService{
			appRepo:           tt.AppRepo,
			eventRepo:         tt.EventRepo,
			eventDeliveryRepo: tt.EventDeliveryRepo,
			eventQueue:        tt.EventQueue,
		}

		t.Run(tt.Name, func(t *testing.T) {
			_, _, err := a.LoadApplicationsPaged(context.TODO(), tt.Uid, tt.Query, tt.Pageable)

			if tt.WantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
