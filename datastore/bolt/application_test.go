package bolt

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Test_LoadApplicationsPaged(t *testing.T) {
	type Group struct {
		UID      string
		Name     string
		AppCount int
	}

	type Expected struct {
		AppCount       int
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		gid      string
		pageData datastore.Pageable
		q        string
		groups   []Group
		expected Expected
	}{
		{
			name:     "No Group ID",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			groups: []Group{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 10,
				},
			},
			gid: "",
			expected: Expected{
				AppCount:       3,
				paginationData: datastore.PaginationData{Total: 12, TotalPage: 4, Page: 1, PerPage: 3, Prev: 0, Next: 2},
			},
		},
		{
			name:     "Filtering using Group ID",
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			gid:      "uid-1",
			q:        "",
			groups: []Group{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 10,
				},
				{
					UID:      "uid-2",
					Name:     "Group 2",
					AppCount: 5,
				},
				{
					UID:      "uid-3",
					Name:     "Group 3",
					AppCount: 15,
				},
			},
			expected: Expected{
				AppCount:       3,
				paginationData: datastore.PaginationData{Total: 12, TotalPage: 4, Page: 1, PerPage: 3, Prev: 0, Next: 2},
			},
		},
		{
			name:     "Filtering using Group ID - Total less than PerPage",
			pageData: datastore.Pageable{Page: 1, PerPage: 10},
			gid:      "uid-1",
			q:        "",
			groups: []Group{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 5,
				},
				{
					UID:      "uid-2",
					Name:     "Group 2",
					AppCount: 3,
				},
				{
					UID:      "uid-3",
					Name:     "Group 3",
					AppCount: 1,
				},
			},
			expected: Expected{
				AppCount:       7,
				paginationData: datastore.PaginationData{Total: 7, TotalPage: 1, Page: 1, PerPage: 10, Prev: 0, Next: 2},
			},
		},
		{
			name:     "Filtering using only title",
			pageData: datastore.Pageable{Page: 1, PerPage: 10},
			gid:      "",
			q:        "App",
			groups: []Group{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 5,
				},
				{
					UID:      "uid-2",
					Name:     "Group 2",
					AppCount: 3,
				},
				{
					UID:      "uid-3",
					Name:     "Group 3",
					AppCount: 2,
				},
			},
			expected: Expected{
				AppCount:       10,
				paginationData: datastore.PaginationData{Total: 10, TotalPage: 1, Page: 1, PerPage: 10, Prev: 0, Next: 2},
			},
		},
		{
			name:     "Filtering using Title and Group ID",
			pageData: datastore.Pageable{Page: 1, PerPage: 10},
			gid:      "uid-2",
			q:        "v",
			groups: []Group{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 5,
				},
				{
					UID:      "uid-2",
					Name:     "Group 2",
					AppCount: 3,
				},
				{
					UID:      "uid-3",
					Name:     "Group 3",
					AppCount: 1,
				},
			},
			expected: Expected{
				AppCount:       2,
				paginationData: datastore.PaginationData{Total: 2, TotalPage: 1, Page: 1, PerPage: 10, Prev: 0, Next: 2},
			},
		},
		{
			name:     "Filtering using Title and Group ID Again",
			pageData: datastore.Pageable{Page: 1, PerPage: 10},
			gid:      "uid-2",
			q:        "1",
			groups: []Group{
				{
					UID:      "uid-1",
					Name:     "Group 1",
					AppCount: 5,
				},
				{
					UID:      "uid-2",
					Name:     "Group 2",
					AppCount: 3,
				},
				{
					UID:      "uid-3",
					Name:     "Group 3",
					AppCount: 1,
				},
			},
			expected: Expected{
				AppCount:       1,
				paginationData: datastore.PaginationData{Total: 1, TotalPage: 1, Page: 1, PerPage: 10, Prev: 0, Next: 2},
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
			for _, g := range tc.groups {
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

			// obvious apps for filter
			a := &datastore.Application{
				Title:   "David",
				GroupID: tc.gid,
				UID:     uuid.NewString(),
			}
			require.NoError(t, appRepo.CreateApplication(context.Background(), a))

			b := &datastore.Application{
				Title:   "Villan",
				GroupID: tc.gid,
				UID:     uuid.NewString(),
			}
			require.NoError(t, appRepo.CreateApplication(context.Background(), b))

			apps, data, err := appRepo.LoadApplicationsPaged(context.Background(), tc.gid, tc.q, tc.pageData)

			require.NoError(t, err)

			require.Equal(t, tc.expected.AppCount, len(apps))

			require.Equal(t, tc.expected.paginationData.Total, data.Total)
			require.Equal(t, tc.expected.paginationData.TotalPage, data.TotalPage)

			require.Equal(t, tc.expected.paginationData.Next, data.Next)
			require.Equal(t, tc.expected.paginationData.Prev, data.Prev)

			require.Equal(t, tc.expected.paginationData.Page, data.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, data.PerPage)
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
		params   datastore.SearchParams
		expected Expected
	}{
		{
			name:   "No Search Params",
			params: datastore.SearchParams{},
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
			params: datastore.SearchParams{CreatedAtStart: times[4].Unix()},
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
			params: datastore.SearchParams{CreatedAtEnd: times[4].Unix()},
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
			params: datastore.SearchParams{CreatedAtStart: times[4].Unix(), CreatedAtEnd: times[6].Unix()},
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
			params: datastore.SearchParams{CreatedAtStart: times[6].Unix(), CreatedAtEnd: times[4].Unix()},
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
