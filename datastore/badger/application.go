package badger

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/timshannon/badgerhold/v4"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type appRepo struct {
	db *badgerhold.Store
}

func (*appRepo) CreateApplicationEndpoint(context.Context, string, string, *datastore.Endpoint) error {
	return nil
}

func NewApplicationRepo(db *badgerhold.Store) datastore.ApplicationRepository {
	return &appRepo{db: db}
}

func (a *appRepo) CreateApplication(ctx context.Context, app *datastore.Application, groupID string) error {
	err := a.assertUniqueAppTitle(ctx, app, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateAppName) {
			return err
		}

		return fmt.Errorf("failed to check if application name is unique: %v", err)
	}

	return a.db.Upsert(app.UID, app)
}

func (a *appRepo) UpdateApplication(ctx context.Context, app *datastore.Application, groupID string) error {
	err := a.assertUniqueAppTitle(ctx, app, groupID)
	if err != nil {
		if errors.Is(err, datastore.ErrDuplicateAppName) {
			return err
		}

		return fmt.Errorf("failed to check if application name is unique: %v", err)
	}

	return a.db.Update(app.UID, app)
}

func (a *appRepo) LoadApplicationsPaged(ctx context.Context, gid, q string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	var apps []datastore.Application = make([]datastore.Application, 0)

	page := pageable.Page
	perPage := pageable.PerPage
	data := datastore.PaginationData{}

	if pageable.Page < 1 {
		page = 1
	}

	if pageable.PerPage < 1 {
		perPage = 10
	}

	prevPage := page - 1
	lowerBound := perPage * prevPage

	af := &appFilter{
		hasTitle:   !util.IsStringEmpty(q),
		hasGroupId: !util.IsStringEmpty(gid),
		title:      q,
		groupId:    gid,
	}

	qry := a.generateQuery(af).Skip(lowerBound).Limit(perPage).SortBy("CreatedAt")
	if pageable.Sort == -1 {
		qry.Reverse()
	}

	err := a.db.Find(&apps, qry)

	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	for i := range apps {
		ap := &apps[i]
		if ap.Endpoints == nil {
			ap.Endpoints = []datastore.Endpoint{}
		}

		count, err := a.db.Count(datastore.Event{}, badgerhold.Where("AppMetadata.UID").Eq(ap.UID))
		if err != nil {
			return nil, datastore.PaginationData{}, err
		}
		ap.Events = int64(count)
	}

	total, err := a.db.Count(&datastore.Application{}, a.generateQuery(af))
	if err != nil {
		return nil, datastore.PaginationData{}, err
	}

	data.TotalPage = int64(math.Ceil(float64(total) / float64(perPage)))
	data.Total = int64(total)

	data.PerPage = int64(perPage)
	data.Next = int64(page + 1)
	data.Page = int64(page)
	data.Prev = int64(prevPage)

	return apps, data, err
}

func (a *appRepo) assertUniqueAppTitle(ctx context.Context, app *datastore.Application, groupID string) error {
	count, err := a.db.Count(
		&datastore.Application{},
		badgerhold.Where("Title").Eq(app.Title).
			And("UID").Ne(app.UID).
			And("GroupID").Eq(groupID).
			And("DocumentStatus").Eq(datastore.ActiveDocumentStatus),
	)

	if err != nil {
		return err
	}

	if count != 0 {
		return datastore.ErrDuplicateAppName
	}

	return nil
}

func (a *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, gid string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	return a.LoadApplicationsPaged(ctx, gid, "", pageable)
}

func (a *appRepo) SearchApplicationsByGroupId(ctx context.Context, gid string, searchParams datastore.SearchParams) ([]datastore.Application, error) {
	var apps []datastore.Application

	af := &appFilter{
		hasGroupId:   !util.IsStringEmpty(gid),
		groupId:      gid,
		hasStartDate: searchParams.CreatedAtStart > 0,
		hasEndDate:   searchParams.CreatedAtEnd > 0,
		searchParams: searchParams,
	}

	err := a.db.Find(&apps, a.generateQuery(af))

	return apps, err
}

func (a *appRepo) FindApplicationByID(ctx context.Context, aid string) (*datastore.Application, error) {
	var application *datastore.Application

	err := a.db.Get(aid, &application)

	if err != nil && errors.Is(err, badgerhold.ErrNotFound) {
		return application, datastore.ErrApplicationNotFound
	}

	return application, err
}

func (a *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*datastore.Endpoint, error) {
	var endpoint *datastore.Endpoint
	var application *datastore.Application

	err := a.db.Get(appID, &application)

	if err != nil && errors.Is(err, badgerhold.ErrNotFound) {
		return endpoint, datastore.ErrApplicationNotFound
	}

	for _, a := range application.Endpoints {
		if a.UID == endpointID {
			endpoint = &a
		}
	}

	if endpoint == nil {
		return nil, datastore.ErrEndpointNotFound
	}

	return endpoint, err
}

func (a *appRepo) DeleteApplication(ctx context.Context, app *datastore.Application) error {
	return a.db.Delete(app.UID, app)
}

func (a *appRepo) DeleteGroupApps(ctx context.Context, gid string) error {
	return a.db.DeleteMatching(&datastore.Application{}, badgerhold.Where("GroupID").Eq(gid))
}

func (a *appRepo) CountGroupApplications(ctx context.Context, gid string) (int64, error) {
	af := &appFilter{hasGroupId: !util.IsStringEmpty(gid), groupId: gid}

	count, err := a.db.Count(&datastore.Application{}, a.generateQuery(af))

	return int64(count), err
}

type appFilter struct {
	hasTitle     bool
	hasGroupId   bool
	hasStartDate bool
	hasEndDate   bool
	title        string
	groupId      string
	searchParams datastore.SearchParams
}

func (a *appRepo) generateQuery(f *appFilter) *badgerhold.Query {
	qFunc := badgerhold.Where

	if f.hasTitle {
		qFunc = qFunc("Title").MatchFunc(func(ra *badgerhold.RecordAccess) (bool, error) {
			field := ra.Field()
			_, ok := field.(string)
			if !ok {
				return false, fmt.Errorf("Field not a string, it's a %T!", field)
			}

			return strings.Contains(strings.ToLower(field.(string)), strings.ToLower(f.title)), nil
		}).And
	}

	if f.hasGroupId {
		qFunc = qFunc("GroupID").Eq(f.groupId).And
	}

	if f.hasStartDate {
		createdStart := primitive.NewDateTimeFromTime(time.Unix(f.searchParams.CreatedAtStart, 0))
		qFunc = qFunc("CreatedAt").Ge(createdStart).And
	}

	if f.hasEndDate {
		createdEnd := primitive.NewDateTimeFromTime(time.Unix(f.searchParams.CreatedAtEnd, 0))
		qFunc = qFunc("CreatedAt").Le(createdEnd).And
	}

	return qFunc("UID").Ne("")
}
