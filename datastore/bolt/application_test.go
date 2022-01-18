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
	type Args struct {
		UID      string
		Name     string
		AppCount int
		pageData models.Pageable
	}

	type Expected struct {
		group_id       string
		paginationData models.PaginationData
	}

	tests := []struct {
		name     string
		args     []Args
		expected []Expected
	}{
		{
			name: "No Group ID",
			args: []Args{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 10,
					pageData: models.Pageable{Page: 1, PerPage: 3},
				},
			},
			expected: []Expected{
				{
					group_id:       "",
					paginationData: models.PaginationData{Total: 10, TotalPage: 4, Page: 1, PerPage: 3, Prev: 0, Next: 2},
				},
			},
		},
		{
			name: "Filtering using Group ID",
			args: []Args{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 10,
					pageData: models.Pageable{Page: 1, PerPage: 3},
				},
				{
					UID:      "uid-2",
					Name:     "Group 2",
					AppCount: 5,
					pageData: models.Pageable{Page: 2, PerPage: 3},
				},
				{
					UID:      "uid-3",
					Name:     "Group 3",
					AppCount: 15,
					pageData: models.Pageable{Page: 3, PerPage: 6},
				},
			},
			expected: []Expected{
				{
					group_id:       "uid-1",
					paginationData: models.PaginationData{Total: 10, TotalPage: 4, Page: 1, PerPage: 3, Prev: 0, Next: 2},
				},
				{
					group_id:       "uid-2",
					paginationData: models.PaginationData{Total: 5, TotalPage: 2, Page: 2, PerPage: 3, Prev: 1, Next: 3},
				},
				{
					group_id:       "uid-2",
					paginationData: models.PaginationData{Total: 15, TotalPage: 3, Page: 3, PerPage: 6, Prev: 2, Next: 4},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			groupRepo := NewGroupRepo(db)
			appRepo := NewApplicationRepo(db)

			// create the groups and group applications
			for _, g := range tc.args {
				require.NoError(t, groupRepo.CreateGroup(context.Background(), &datastore.Group{UID: g.UID, Name: g.Name}))

				for i := 0; i < g.AppCount; i++ {
					a := &datastore.Application{
						Title:   fmt.Sprintf("Application %v", i),
						GroupID: g.UID,
						UID:     uuid.NewString(),
					}
					require.NoError(t, appRepo.CreateApplication(context.Background(), a))
				}
			}

			for i, g := range tc.args {
				if g.UID == tc.expected[i].group_id {
					_, pageData, err := appRepo.LoadApplicationsPaged(context.Background(), tc.args[i].UID,
						models.Pageable{Page: tc.args[i].pageData.Page, PerPage: tc.args[i].pageData.PerPage})

					require.NoError(t, err)

					require.Equal(t, pageData.TotalPage, tc.expected[i].paginationData.TotalPage)
					require.Equal(t, pageData.Next, tc.expected[i].paginationData.Next)
					require.Equal(t, pageData.Page, tc.expected[i].paginationData.Page)
					require.Equal(t, pageData.PerPage, tc.expected[i].paginationData.PerPage)
					require.Equal(t, pageData.Prev, tc.expected[i].paginationData.Prev)
					require.Equal(t, pageData.Total, tc.expected[i].paginationData.Total)
				}
			}
		})
	}
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
	type Args struct {
		uid      string
		name     string
		appCount int
	}

	type Expected struct {
		apps int
	}

	times := []time.Time{
		time.Date(2020, time.November, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 4, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 9, 0, 0, 0, 0, time.UTC),
		time.Date(2020, time.November, 10, 0, 0, 0, 0, time.UTC),
	}

	tests := []struct {
		name     string
		args     []Args
		params   models.SearchParams
		expected Expected
	}{
		{
			name:   "No Search Params",
			params: models.SearchParams{},
			args: []Args{
				{
					uid:      "uid-1",
					name:     "Group 1",
					appCount: 10,
				},
			},
			expected: Expected{
				apps: 10,
			},
		},
		{
			name:   "Search Params - Only CreatedAtStart",
			params: models.SearchParams{CreatedAtStart: times[4].Unix()},
			args: []Args{
				{
					uid:      "uid-1",
					name:     "Group 1",
					appCount: 10,
				},
				{
					uid:      "uid-2",
					name:     "Group 2",
					appCount: 10,
				},
			},
			expected: Expected{
				apps: 6,
			},
		},
		{
			name:   "Search Params - Only CreatedAtEnd",
			params: models.SearchParams{CreatedAtEnd: times[4].Unix()},
			args: []Args{
				{
					uid:      "uid-1",
					name:     "Group 1",
					appCount: 10,
				},
				{
					uid:      "uid-2",
					name:     "Group 2",
					appCount: 10,
				},
			},
			expected: Expected{
				apps: 5,
			},
		},
		{
			name:   "Search Params - CreatedAtEnd and CreatedAtEnd (Valid interval)",
			params: models.SearchParams{CreatedAtStart: times[4].Unix(), CreatedAtEnd: times[6].Unix()},
			args: []Args{
				{
					uid:      "uid-1",
					name:     "Group 1",
					appCount: 10,
				},
				{
					uid:      "uid-2",
					name:     "Group 2",
					appCount: 10,
				},
			},
			expected: Expected{
				apps: 3,
			},
		},
		{
			name:   "Search Params - CreatedAtEnd and CreatedAtEnd (Invalid interval)",
			params: models.SearchParams{CreatedAtStart: times[6].Unix(), CreatedAtEnd: times[4].Unix()},
			args: []Args{
				{
					uid:      "uid-1",
					name:     "Group 1",
					appCount: 10,
				},
				{
					uid:      "uid-2",
					name:     "Group 2",
					appCount: 10,
				},
			},
			expected: Expected{
				apps: 0,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			groupRepo := NewGroupRepo(db)
			appRepo := NewApplicationRepo(db)

			for _, g := range tc.args {
				require.NoError(t, groupRepo.CreateGroup(context.Background(), &datastore.Group{UID: g.uid, Name: g.name}))

				for i := 0; i < g.appCount; i++ {
					a := &datastore.Application{
						Title:     fmt.Sprintf("Application %v", i),
						GroupID:   g.uid,
						UID:       uuid.NewString(),
						CreatedAt: primitive.NewDateTimeFromTime(times[i]),
					}
					require.NoError(t, appRepo.CreateApplication(context.Background(), a))
				}
			}

			apps, err := appRepo.SearchApplicationsByGroupId(context.Background(), tc.args[0].uid, tc.params)
			require.NoError(t, err)
			require.Equal(t, tc.expected.apps, len(apps))
		})
	}
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
