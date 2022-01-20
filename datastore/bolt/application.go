package bolt

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type appRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewApplicationRepo(db *bbolt.DB) datastore.ApplicationRepository {
	bucketName := "applications"
	err := db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})

	if err != nil {
		return nil
	}

	return &appRepo{db: db, bucketName: bucketName}
}

func (a *appRepo) CreateApplication(ctx context.Context, app *datastore.Application) error {
	return a.createUpdateApplication(app)
}

func (a *appRepo) UpdateApplication(ctx context.Context, app *datastore.Application) error {
	return a.createUpdateApplication(app)
}

func (a *appRepo) LoadApplicationsPaged(ctx context.Context, gid, q string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	var apps []datastore.Application = make([]datastore.Application, 0)

	page := pageable.Page
	prevPage := pageable.Page
	perPage := pageable.PerPage
	data := datastore.PaginationData{}

	if pageable.Page < 1 {
		page = 1
	}

	if pageable.PerPage < 1 {
		perPage = 10
	}

	if page < 1 {
		prevPage = 1
	} else {
		prevPage = page - 1
	}

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		c := b.Cursor()
		i := 1

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if v == nil {
				continue
			}

			if i > perPage*prevPage && i <= perPage*page {
				var app datastore.Application
				err := json.Unmarshal(v, &app)
				if err != nil {
					return err
				}

				shouldAdd := false 

				if !util.IsStringEmpty(gid) && app.GroupID == gid {
					shouldAdd = true

					if !util.IsStringEmpty(q) && !strings.Contains(app.Title, q) {
						shouldAdd = false
					} 
				} else if util.IsStringEmpty(gid){
					shouldAdd = true

					if !util.IsStringEmpty(q) && !strings.Contains(app.Title, q) {
						shouldAdd = false
					} 
				}

				if shouldAdd {
					apps = append(apps, app)
				}
			}
			i++

			if i == (perPage*page)+perPage {
				break
			}
		}

		if util.IsStringEmpty(gid) {
			data.TotalPage = int64(math.Ceil(float64(b.Stats().KeyN) / float64(perPage)))
			data.Total = int64(b.Stats().KeyN)
		} else {
			total, err := a.CountGroupApplications(ctx, gid)
			if err != nil {
				return nil
			}

			println(total)

			data.TotalPage = int64(math.Ceil(float64(total) / float64(perPage)))
			data.Total = int64(total)
		}

		data.PerPage = int64(perPage)
		data.Next = int64(page + 1)
		data.Page = int64(page)
		data.Prev = int64(prevPage)

		return nil
	})

	return apps, data, err
}

func (a *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, gid string, pageable datastore.Pageable) ([]datastore.Application, datastore.PaginationData, error) {
	return a.LoadApplicationsPaged(ctx, gid, "", pageable)
}

func (a *appRepo) SearchApplicationsByGroupId(ctx context.Context, gid string, searchParams datastore.SearchParams) ([]datastore.Application, error) {
	var apps []datastore.Application
	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var app datastore.Application
			err := json.Unmarshal(v, &app)
			if err != nil {
				return err
			}

			shouldAdd := false
			if app.GroupID == gid {
				shouldAdd = true

				if searchParams.CreatedAtStart != 0 && app.CreatedAt.Time().Unix() < searchParams.CreatedAtStart {
					shouldAdd = false
				}

				if searchParams.CreatedAtEnd != 0 && app.CreatedAt.Time().Unix() > searchParams.CreatedAtEnd {
					shouldAdd = false
				}
			}

			if shouldAdd {
				apps = append(apps, app)
			}

			return nil
		})
	})

	return apps, err
}

func (a *appRepo) FindApplicationByID(ctx context.Context, aid string) (*datastore.Application, error) {
	var application *datastore.Application
	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		appBytes := b.Get([]byte(aid))
		if appBytes == nil {
			return datastore.ErrApplicationNotFound
		}

		var app *datastore.Application
		err := json.Unmarshal(appBytes, &app)
		if err != nil {
			return err
		}
		application = app

		return nil
	})

	return application, err
}

func (a *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*datastore.Endpoint, error) {
	var endpoint *datastore.Endpoint
	err := a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		appBytes := b.Get([]byte(appID))
		if appBytes == nil {
			return datastore.ErrApplicationNotFound
		}

		var app *datastore.Application
		err := json.Unmarshal(appBytes, &app)
		if err != nil {
			return err
		}

		for _, v := range app.Endpoints {
			if v.UID == endpointID {
				endpoint = &v
			}
		}

		return nil
	})

	return endpoint, err
}

func (a *appRepo) DeleteApplication(ctx context.Context, app *datastore.Application) error {
	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		return b.Delete([]byte(app.UID))
	})
}

func (a *appRepo) UpdateApplicationEndpointsStatus(ctx context.Context, aid string, endpointIds []string, status datastore.EndpointStatus) error {
	endpointMap := make(map[string]bool)
	for i := 0; i < len(endpointIds); i++ {
		endpointMap[endpointIds[i]] = true
	}

	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		appBytes := b.Get([]byte(aid))
		if appBytes == nil {
			return datastore.ErrApplicationNotFound
		}

		var app *datastore.Application
		err := json.Unmarshal(appBytes, &app)
		if err != nil {
			return err
		}

		for i := 0; i < len(app.Endpoints); i++ {
			if _, ok := endpointMap[app.Endpoints[i].UID]; ok {
				app.Endpoints[i].Status = status
			}
		}
		app.UpdatedAt = primitive.NewDateTimeFromTime(time.Now())

		aJson, err := json.Marshal(app)
		if err != nil {
			return err
		}

		pErr := b.Put([]byte(aid), aJson)
		if pErr != nil {
			return pErr
		}

		return nil
	})
}

func (a *appRepo) DeleteGroupApps(ctx context.Context, gid string) error {
	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var app *datastore.Application
			err := json.Unmarshal(v, &app)
			if err != nil {
				return err
			}

			if app.GroupID == gid {
				err := b.Delete([]byte(app.UID))
				if err != nil {
					return err
				}
			}

			return nil
		})
	})
}

func (a *appRepo) CountGroupApplications(ctx context.Context, gid string) (int64, error) {
	count := int64(0)
	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var app *datastore.Application
			err := json.Unmarshal(v, &app)
			if err != nil {
				return err
			}

			if app.GroupID == gid {
				count += 1
			}

			return nil
		})
	})

	return count, err
}

func (a *appRepo) createUpdateApplication(app *datastore.Application) error {
	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		aJson, err := json.Marshal(app)
		if err != nil {
			return err
		}

		pErr := b.Put([]byte(app.UID), aJson)
		if pErr != nil {
			return pErr
		}

		return nil
	})
}
