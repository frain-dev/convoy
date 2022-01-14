package bolt

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	"go.etcd.io/bbolt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type appRepo struct {
	db         *bbolt.DB
	bucketName string
}

func NewApplicationRepo(db *bbolt.DB) convoy.ApplicationRepository {
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

func (a *appRepo) CreateApplication(ctx context.Context, app *convoy.Application) error {
	return a.createUpdateApplication(app)
}

func (a *appRepo) UpdateApplication(ctx context.Context, app *convoy.Application) error {
	return a.createUpdateApplication(app)
}

func (a *appRepo) LoadApplicationsPaged(ctx context.Context, gid string, pageable models.Pageable) ([]convoy.Application, models.PaginationData, error) {
	var apps []convoy.Application = make([]convoy.Application, 0)
	data := models.PaginationData{}
	prevPage := pageable.Page

	if pageable.Page == 0 {
		prevPage = 0
	} else {
		prevPage = pageable.Page - 1
	}

	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		c := b.Cursor()
		i := 0

		for k, v := c.First(); k != nil; k, v = c.Next() {
			if k == nil || v == nil {
				continue
			}

			if i >= pageable.PerPage*prevPage && i < pageable.PerPage*pageable.Page {
				var app convoy.Application
				err := json.Unmarshal(v, &app)
				if err != nil {
					return err
				}

				if !util.IsStringEmpty(gid) {
					if app.GroupID == gid {
						apps = append(apps, app)
					}
				} else {
					apps = append(apps, app)
				}
			}
			i++

			if i == pageable.PerPage*pageable.Page {
				break
			}
		}

		data.TotalPage = int64(math.Ceil(float64(b.Stats().KeyN) / float64(pageable.PerPage)))
		data.PerPage = int64(pageable.PerPage)
		data.Next = int64(pageable.Page + 1)
		data.Total = int64(b.Stats().KeyN)
		data.Page = int64(pageable.Page)
		data.Prev = int64(prevPage)

		return nil
	})

	return apps, data, err
}

func (a *appRepo) LoadApplicationsPagedByGroupId(ctx context.Context, gid string, pageable models.Pageable) ([]convoy.Application, models.PaginationData, error) {
	return a.LoadApplicationsPaged(ctx, gid, pageable)
}

func (a *appRepo) SearchApplicationsByGroupId(ctx context.Context, gid string, searchParams models.SearchParams) ([]convoy.Application, error) {
	var apps []convoy.Application
	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var app convoy.Application
			err := json.Unmarshal(v, &app)
			if err != nil {
				return err
			}

			if app.GroupID == gid {
				apps = append(apps, app)
			}

			return nil
		})
	})

	return apps, err
}

func (a *appRepo) FindApplicationByID(ctx context.Context, aid string) (*convoy.Application, error) {
	var application *convoy.Application
	err := a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		appBytes := b.Get([]byte(aid))
		if appBytes == nil {
			return convoy.ErrApplicationNotFound
		}

		var app *convoy.Application
		err := json.Unmarshal(appBytes, &app)
		if err != nil {
			return err
		}
		application = app

		return nil
	})

	return application, err
}

func (a *appRepo) FindApplicationEndpointByID(ctx context.Context, appID string, endpointID string) (*convoy.Endpoint, error) {
	var endpoint *convoy.Endpoint
	err := a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		appBytes := b.Get([]byte(appID))
		if appBytes == nil {
			return convoy.ErrApplicationNotFound
		}

		var app *convoy.Application
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

func (a *appRepo) DeleteApplication(ctx context.Context, app *convoy.Application) error {
	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))
		return b.Delete([]byte(app.UID))
	})
}

func (a *appRepo) UpdateApplicationEndpointsStatus(ctx context.Context, aid string, endpointIds []string, status convoy.EndpointStatus) error {
	endpointMap := make(map[string]bool)
	for i := 0; i < len(endpointIds); i++ {
		endpointMap[endpointIds[i]] = true
	}

	return a.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		appBytes := b.Get([]byte(aid))
		if appBytes == nil {
			return convoy.ErrApplicationNotFound
		}

		var app *convoy.Application
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
	return a.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(a.bucketName))

		return b.ForEach(func(k, v []byte) error {
			var app *convoy.Application
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
			var app *convoy.Application
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

func (a *appRepo) createUpdateApplication(app *convoy.Application) error {
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
