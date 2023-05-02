//go:build integration
// +build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/datastore"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
)

func Test_CreateMetaEvent(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	metaEventRepo := NewMetaEventRepo(db)
	metaEvent := generateMetaEvent(t, db)
	ctx := context.Background()

	require.NoError(t, metaEventRepo.CreateMetaEvent(ctx, metaEvent))

	newMetaEvent, err := metaEventRepo.FindMetaEventByID(ctx, metaEvent.ProjectID, metaEvent.UID)

	require.NoError(t, err)

	err = metaEventRepo.UpdateMetaEvent(ctx, metaEvent.ProjectID, metaEvent)
	require.NoError(t, err)

	newMetaEvent.CreatedAt = time.Time{}
	newMetaEvent.UpdatedAt = time.Time{}
	metaEvent.CreatedAt, metaEvent.UpdatedAt = time.Time{}, time.Time{}

	require.Equal(t, metaEvent, newMetaEvent)
}

func generateMetaEvent(t *testing.T, db database.Database) *datastore.MetaEvent {
	project := seedProject(t, db)

	return &datastore.MetaEvent{
		UID:       ulid.Make().String(),
		EventType: string(datastore.EndpointCreated),
		ProjectID: project.UID,
		Metadata:  &datastore.Metadata{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
