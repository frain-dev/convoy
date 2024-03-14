//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/database"

	ncache "github.com/frain-dev/convoy/cache/noop"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
)

func Test_eventCatalogueRepo_CreateEventCatalogue(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	catalogueRepo := NewEventCatalogueRepo(db, &ncache.NoopCache{})
	p := seedProject(t, db)
	c := &datastore.EventCatalogue{
		UID:         ulid.Make().String(),
		ProjectID:   p.UID,
		Type:        datastore.OpenAPICatalogueType,
		Events:      nil,
		OpenAPISpec: []byte(`tejbdojer9juj`),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := catalogueRepo.CreateEventCatalogue(context.Background(), c)
	require.NoError(t, err)

	event := seedEvent(t, db, p)
	c1 := &datastore.EventCatalogue{
		UID:       ulid.Make().String(),
		ProjectID: p.UID,
		Type:      datastore.EventsDataCatalogueType,
		Events: datastore.EventDataCatalogues{
			{
				Name:        "item.paid",
				Description: "Details of a paid item",
				EventID:     event.UID,
				Data:        event.Data,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = catalogueRepo.CreateEventCatalogue(context.Background(), c1)
	require.Equal(t, ErrEventCatalogueExists, err)

	dbCatalogue, err := catalogueRepo.FindEventCatalogueByProjectID(context.Background(), p.UID)
	require.NoError(t, err)

	c.CreatedAt, c.UpdatedAt = time.Time{}, time.Time{}
	dbCatalogue.CreatedAt, dbCatalogue.UpdatedAt = time.Time{}, time.Time{}
	require.Equal(t, c, dbCatalogue)
}

func Test_eventCatalogueRepo_DeleteEventCatalogue(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	c := seedCatalogue(db, t)

	catalogueRepo := NewEventCatalogueRepo(db, &ncache.NoopCache{})
	err := catalogueRepo.DeleteEventCatalogue(context.Background(), c.UID, c.ProjectID)
	require.NoError(t, err)

	_, err = catalogueRepo.FindEventCatalogueByProjectID(context.Background(), c.ProjectID)
	require.Equal(t, datastore.ErrCatalogueNotFound, err)
}

func seedCatalogue(db database.Database, t *testing.T) *datastore.EventCatalogue {
	catalogueRepo := NewEventCatalogueRepo(db, &ncache.NoopCache{})
	p := seedProject(t, db)
	c := &datastore.EventCatalogue{
		UID:         ulid.Make().String(),
		ProjectID:   p.UID,
		Type:        datastore.OpenAPICatalogueType,
		Events:      nil,
		OpenAPISpec: []byte(`tejbdojer9juj`),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	err := catalogueRepo.CreateEventCatalogue(context.Background(), c)
	require.NoError(t, err)

	return c
}

func Test_eventCatalogueRepo_UpdateEventCatalogue(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()
	c := seedCatalogue(db, t)

	b := []byte(`yaml`)
	ev := datastore.EventDataCatalogues{
		{
			Name:        "item.paid",
			Description: "descibed",
			EventID:     "00232390",
			Data:        []byte(`{}`),
		},
	}

	c.OpenAPISpec = b
	c.Events = ev

	catalogueRepo := NewEventCatalogueRepo(db, &ncache.NoopCache{})
	err := catalogueRepo.UpdateEventCatalogue(context.Background(), c)
	require.NoError(t, err)

	dbCatalogue, err := catalogueRepo.FindEventCatalogueByProjectID(context.Background(), c.ProjectID)
	require.NoError(t, err)

	require.Equal(t, b, dbCatalogue.OpenAPISpec)
	require.Equal(t, ev, dbCatalogue.Events)
}
