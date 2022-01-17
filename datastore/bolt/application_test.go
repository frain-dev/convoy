package bolt

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/models"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func Test_LoadApplicationsPaged(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newOrg := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	for i := 0; i < 10; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: newOrg.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	_, pageData, err := appRepo.LoadApplicationsPaged(context.Background(), "", models.Pageable{
		Page:    1,
		PerPage: 3,
	})

	require.NoError(t, err)

	require.Equal(t, pageData.TotalPage, int64(4))
}

func Test_LoadApplicationsPaged_GroupIdFilter(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	group1 := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	group2 := &datastore.Group{
		Name: "Group 2",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), group1))

	for i := 0; i < 10; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: group1.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	for i := 0; i < 5; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: group2.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	apps1, pageData, e := appRepo.LoadApplicationsPaged(context.Background(), group1.UID, models.Pageable{
		Page:    2,
		PerPage: 3,
	})

	require.NoError(t, e)

	for _, v := range apps1 {
		require.Equal(t, group1.UID, v.GroupID)
	}

	require.Equal(t, int64(10), pageData.Total)
	require.Equal(t, int64(2), pageData.Page)
	require.Equal(t, int64(3), pageData.PerPage)
	require.Equal(t, int64(1), pageData.Prev)
	require.Equal(t, int64(3), pageData.Next)
	require.Equal(t, int64(4), pageData.TotalPage)

	apps2, pageData, err := appRepo.LoadApplicationsPaged(context.Background(), group2.UID, models.Pageable{
		Page:    1,
		PerPage: 3,
	})

	require.NoError(t, err)

	for _, v := range apps2 {
		require.Equal(t, group2.UID, v.GroupID)
	}

	require.Equal(t, int64(5), pageData.Total)
	require.Equal(t, int64(1), pageData.Page)
	require.Equal(t, int64(3), pageData.PerPage)
	require.Equal(t, int64(0), pageData.Prev)
	require.Equal(t, int64(2), pageData.Next)
	require.Equal(t, int64(2), pageData.TotalPage)
}

func Test_CreateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newOrg := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newOrg))

	app := &datastore.Application{
		Title:   "Application 1",
		GroupID: newOrg.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))
}

func Test_UpdateApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	newGroup := &datastore.Group{
		Name: "Random new group",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		UID:     uuid.NewString(),
		Title:   "Next application name",
		GroupID: newGroup.UID,
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	newTitle := "Newer name"

	app.Title = newTitle

	require.NoError(t, appRepo.UpdateApplication(context.Background(), app))

	newApp, err := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.NoError(t, err)

	require.Equal(t, newTitle, newApp.Title)
}

func Test_FindApplicationByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	groupRepo := NewGroupRepo(db)

	newGroup := &datastore.Group{
		UID:  uuid.NewString(),
		Name: "Random Group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		Title:   "Application 10",
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	_, err := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.Error(t, err)

	require.True(t, errors.Is(err, datastore.ErrApplicationNotFound))

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	_, e := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.NoError(t, e)
}

func Test_SearchApplicationsByGroupId(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	groupOne := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	groupTwo := &datastore.Group{
		Name: "Group 2",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupOne))
	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupTwo))

	for i := 0; i < 4; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: groupOne.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	for i := 0; i < 5; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: groupTwo.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	groupOneapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupOne.UID, models.SearchParams{})
	require.NoError(t, err)

	groupTwoapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupTwo.UID, models.SearchParams{})
	require.NoError(t, err)

	require.Equal(t, len(groupOneapps), 4)
	require.Equal(t, len(groupTwoapps), 5)
}

func Test_SearchApplicationsByGroupId_CreatedAtStartDate(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	groupOne := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	groupTwo := &datastore.Group{
		Name: "Group 2",
		UID:  uuid.NewString(),
	}

	times := []time.Time{
		time.Date(2020, time.November, 10, 1, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 2, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 3, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 4, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 5, 0, 0, 0, time.UTC),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupOne))
	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupTwo))

	for i := 0; i < 4; i++ {
		a := &datastore.Application{
			Title:     fmt.Sprintf("Application %v", i),
			GroupID:   groupOne.UID,
			UID:       uuid.NewString(),
			CreatedAt: primitive.NewDateTimeFromTime(times[i]),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	for i := 0; i < 5; i++ {
		a := &datastore.Application{
			Title:     fmt.Sprintf("Application %v", i),
			GroupID:   groupTwo.UID,
			UID:       uuid.NewString(),
			CreatedAt: primitive.NewDateTimeFromTime(times[i]),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	groupOneapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupOne.UID, models.SearchParams{CreatedAtStart: times[1].Unix()})
	require.NoError(t, err)

	groupTwoapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupTwo.UID, models.SearchParams{CreatedAtStart: times[2].Unix()})
	require.NoError(t, err)

	require.Equal(t, len(groupOneapps), 3)
	require.Equal(t, len(groupTwoapps), 3)
}

func Test_SearchApplicationsByGroupId_CreatedAtEndDate(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	groupOne := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	groupTwo := &datastore.Group{
		Name: "Group 2",
		UID:  uuid.NewString(),
	}

	times := []time.Time{
		time.Date(2020, time.November, 10, 1, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 2, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 3, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 4, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 5, 0, 0, 0, time.UTC),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupOne))
	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupTwo))

	for i := 0; i < 4; i++ {
		a := &datastore.Application{
			Title:     fmt.Sprintf("Application %v", i),
			GroupID:   groupOne.UID,
			UID:       uuid.NewString(),
			CreatedAt: primitive.NewDateTimeFromTime(times[i]),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	for i := 0; i < 5; i++ {
		a := &datastore.Application{
			Title:     fmt.Sprintf("Application %v", i),
			GroupID:   groupTwo.UID,
			UID:       uuid.NewString(),
			CreatedAt: primitive.NewDateTimeFromTime(times[i]),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	groupOneapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupOne.UID, models.SearchParams{CreatedAtEnd: times[1].Unix()})
	require.NoError(t, err)

	groupTwoapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupTwo.UID, models.SearchParams{CreatedAtEnd: times[2].Unix()})
	require.NoError(t, err)

	require.Equal(t, 2, len(groupOneapps))
	require.Equal(t, 3, len(groupTwoapps))
}

func Test_SearchApplicationsByGroupId_CreatedAtStartAndEndDate(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	groupOne := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	groupTwo := &datastore.Group{
		Name: "Group 2",
		UID:  uuid.NewString(),
	}

	times := []time.Time{
		time.Date(2020, time.November, 10, 1, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 2, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 3, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 4, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 5, 0, 0, 0, time.UTC),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupOne))
	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupTwo))

	for i := 0; i < 4; i++ {
		a := &datastore.Application{
			Title:     fmt.Sprintf("Application %v", i),
			GroupID:   groupOne.UID,
			UID:       uuid.NewString(),
			CreatedAt: primitive.NewDateTimeFromTime(times[i]),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	for i := 0; i < 5; i++ {
		a := &datastore.Application{
			Title:     fmt.Sprintf("Application %v", i),
			GroupID:   groupTwo.UID,
			UID:       uuid.NewString(),
			CreatedAt: primitive.NewDateTimeFromTime(times[i]),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	groupOneapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupOne.UID, models.SearchParams{CreatedAtEnd: times[3].Unix(), CreatedAtStart: times[2].Unix()})
	require.NoError(t, err)

	groupTwoapps, err := appRepo.SearchApplicationsByGroupId(context.Background(), groupTwo.UID, models.SearchParams{CreatedAtEnd: times[3].Unix(), CreatedAtStart: times[1].Unix()})
	require.NoError(t, err)

	require.Equal(t, 2, len(groupOneapps))
	require.Equal(t, 3, len(groupTwoapps))
}

func Test_DeleteApplication(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	appRepo := NewApplicationRepo(db)

	groupRepo := NewGroupRepo(db)

	newGroup := &datastore.Group{
		UID:  uuid.NewString(),
		Name: "Random Group",
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), newGroup))

	app := &datastore.Application{
		Title:   "Application 10",
		GroupID: newGroup.UID,
		UID:     uuid.NewString(),
	}

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	_, e := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.NoError(t, e)

	require.NoError(t, appRepo.DeleteApplication(context.Background(), app))

	_, err := appRepo.FindApplicationByID(context.Background(), app.UID)
	require.Error(t, err)

	require.True(t, errors.Is(err, datastore.ErrApplicationNotFound))
}

func Test_DeleteGroupApps(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

	groupOne := &datastore.Group{
		Name: "Group 1",
		UID:  uuid.NewString(),
	}

	groupTwo := &datastore.Group{
		Name: "Group 2",
		UID:  uuid.NewString(),
	}

	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupOne))
	require.NoError(t, groupRepo.CreateGroup(context.Background(), groupTwo))

	for i := 0; i < 4; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: groupOne.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	for i := 0; i < 5; i++ {
		a := &datastore.Application{
			Title:   fmt.Sprintf("Application %v", i),
			GroupID: groupTwo.UID,
			UID:     uuid.NewString(),
		}
		require.NoError(t, appRepo.CreateApplication(context.Background(), a))
	}

	count, err := appRepo.CountGroupApplications(context.Background(), groupOne.UID)
	require.NoError(t, err)
	require.Equal(t, int64(4), count)

	require.NoError(t, appRepo.DeleteGroupApps(context.Background(), groupOne.UID))

	count2, err2 := appRepo.CountGroupApplications(context.Background(), groupOne.UID)
	require.NoError(t, err2)
	require.Equal(t, int64(0), count2)

	count3, err3 := appRepo.CountGroupApplications(context.Background(), groupTwo.UID)
	require.NoError(t, err3)
	require.Equal(t, int64(5), count3)
}
