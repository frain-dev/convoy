package bolt

import (
	"context"
	"testing"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const timeFormat = "2006-01-02T15:04:05"

func Test_FindEventByID(t *testing.T) {
	db, closeFn := getDB(t)
	defer closeFn()

	eventRepo := NewEventRepo(db)
	groupRepo := NewGroupRepo(db)
	appRepo := NewApplicationRepo(db)

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

	endpoint := datastore.Endpoint{
		UID:            uuid.New().String(),
		TargetURL:      "https://example.com",
		Description:    "Is this your king?",
		Events:         []string{"king.maker"},
		Secret:         "jkdlifbdskds",
		Status:         datastore.ActiveEndpointStatus,
		CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	app.Endpoints = append(app.Endpoints, endpoint)

	require.NoError(t, appRepo.CreateApplication(context.Background(), app))

	event := &datastore.Event{
		UID:              "eid-1",
		EventType:        datastore.EventType(endpoint.Events[0]),
		MatchedEndpoints: 1,
		Data:             []byte("{\"key\":\"value\"}"),
		CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
		AppMetadata: &datastore.AppMetadata{
			Title:        app.Title,
			UID:          app.UID,
			GroupID:      app.GroupID,
			SupportEmail: app.SupportEmail,
		},
		DocumentStatus: datastore.ActiveDocumentStatus,
	}

	require.NoError(t, eventRepo.CreateEvent(context.Background(), event))

	e, err := eventRepo.FindEventByID(context.Background(), "eid-1")
	require.NoError(t, err)

	require.Equal(t, event.Data, e.Data)
	require.Equal(t, event.EventType, e.EventType)
}

func Test_CountGroupMessages(t *testing.T) {
	type Group struct {
		UID        string
		Name       string
		EventCount int
	}

	tests := []struct {
		name      string
		gid       string
		groups    []Group
		endpoints []datastore.Endpoint
		expected  int64
	}{
		{
			name: "Count Group Messages",
			gid:  "gid-1",
			groups: []Group{
				{
					UID:        "gid-1",
					Name:       "Group 1",
					EventCount: 2,
				},
			},
			endpoints: []datastore.Endpoint{
				{
					UID:            uuid.New().String(),
					TargetURL:      "https://example.com",
					Description:    "Is this your king?",
					Events:         []string{"king.maker"},
					Secret:         "Cut_Onions_Here",
					Status:         datastore.ActiveEndpointStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					UID:            uuid.New().String(),
					TargetURL:      "https://example.com",
					Description:    "No be juju be that?",
					Events:         []string{"king.taker"},
					Secret:         "Some_Awesome_Knife",
					Status:         datastore.ActiveEndpointStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			expected: int64(2),
		},
		{
			name: "Count Group Messages more groups",
			gid:  "gid-2",
			groups: []Group{
				{
					UID:        "gid-1",
					Name:       "Group 1",
					EventCount: 2,
				},
				{
					UID:        "gid-2",
					Name:       "Group 1",
					EventCount: 5,
				},
			},
			endpoints: []datastore.Endpoint{
				{
					UID:            uuid.New().String(),
					TargetURL:      "https://example.com",
					Description:    "Is this your king?",
					Events:         []string{"king.maker"},
					Secret:         "Cut_Onions_Here",
					Status:         datastore.ActiveEndpointStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
				{
					UID:            uuid.New().String(),
					TargetURL:      "https://example.com",
					Description:    "No be juju be that?",
					Events:         []string{"king.taker"},
					Secret:         "Some_Awesome_Knife",
					Status:         datastore.ActiveEndpointStatus,
					CreatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					UpdatedAt:      primitive.NewDateTimeFromTime(time.Now()),
					DocumentStatus: datastore.ActiveDocumentStatus,
				},
			},
			expected: int64(5),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			eventRepo := NewEventRepo(db)
			groupRepo := NewGroupRepo(db)
			appRepo := NewApplicationRepo(db)

			app := &datastore.Application{
				Title:   "My Application",
				GroupID: tc.gid,
				UID:     uuid.NewString(),
			}

			// create the group and group application
			for _, group := range tc.groups {
				require.NoError(t, groupRepo.CreateGroup(context.Background(),
					&datastore.Group{UID: group.UID, Name: group.Name}))

				app.GroupID = group.UID
				app.Endpoints = append(app.Endpoints, tc.endpoints...)
				require.NoError(t, appRepo.CreateApplication(context.Background(), app))

				for i := 0; i < group.EventCount; i++ {
					event := &datastore.Event{
						UID:              uuid.NewString(),
						EventType:        "king.taker",
						MatchedEndpoints: 1,
						Data:             []byte("{\"key\":\"value\"}"),
						CreatedAt:        primitive.NewDateTimeFromTime(time.Now()),
						UpdatedAt:        primitive.NewDateTimeFromTime(time.Now()),
						AppMetadata: &datastore.AppMetadata{
							Title:        app.Title,
							UID:          app.UID,
							GroupID:      app.GroupID,
							SupportEmail: app.SupportEmail,
						},
						DocumentStatus: datastore.ActiveDocumentStatus,
					}

					require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
				}
			}

			count, err := eventRepo.CountGroupMessages(context.Background(), tc.gid)
			require.NoError(t, err)

			require.Equal(t, int64(tc.expected), count)
		})
	}
}

func Test_LoadEventIntervals(t *testing.T) {
	type Group struct {
		UID  string
		Name string
	}

	type Params struct {
		start string
		end   string
	}

	tests := []struct {
		name     string
		group    Group
		times    []string
		params   Params
		period   datastore.Period
		expected []datastore.EventInterval
	}{
		{
			name: "Load Event Intervals - Monthly",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			times: []string{
				"2021-11-01T00:01:20",
				"2021-11-11T00:01:20",
				"2021-11-12T00:01:20",
				"2021-12-12T00:01:20",
				"2022-01-01T00:01:20",
				"2022-01-02T00:01:20",
				"2022-01-12T00:01:20",
			},
			params: Params{start: "2021-11-01T00:00:00", end: "2022-02-01T00:00:00"},
			period: datastore.Monthly,
			expected: []datastore.EventInterval{
				{Data: datastore.EventIntervalData{Interval: int64(11), Time: "2021-11"}, Count: uint64(3)},
				{Data: datastore.EventIntervalData{Interval: int64(12), Time: "2021-12"}, Count: uint64(1)},
				{Data: datastore.EventIntervalData{Interval: int64(1), Time: "2022-01"}, Count: uint64(3)},
			},
		},
		{
			name: "Load Event Intervals - Daily",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			times: []string{
				"2022-01-01T00:01:20",
				"2022-01-01T10:01:20",
				"2021-11-01T20:01:20",
				"2021-11-01T11:01:20",
				"2021-12-12T13:01:20",
				"2022-01-12T01:01:20",
				"2022-01-12T00:01:20",
			},
			params: Params{start: "2021-11-01T00:00:00", end: "2022-02-01T00:00:00"},
			period: datastore.Daily,
			expected: []datastore.EventInterval{
				{Data: datastore.EventIntervalData{Interval: int64(1), Time: "2021-11-01"}, Count: uint64(2)},
				{Data: datastore.EventIntervalData{Interval: int64(12), Time: "2021-12-12"}, Count: uint64(1)},
				{Data: datastore.EventIntervalData{Interval: int64(1), Time: "2022-01-01"}, Count: uint64(2)},
				{Data: datastore.EventIntervalData{Interval: int64(12), Time: "2022-01-12"}, Count: uint64(2)},
			},
		},
		{
			name: "Load Event Intervals - Weekly",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			times: []string{
				"2021-11-01T20:01:20",
				"2021-11-01T11:01:20",
				"2021-12-12T13:01:20",
				"2022-01-01T00:01:20",
				"2022-01-01T00:01:20",
				"2022-01-12T01:01:20",
				"2022-01-12T00:01:20",
			},
			params: Params{start: "2021-11-01T00:00:00", end: "2022-02-01T00:00:00"},
			period: datastore.Weekly,
			expected: []datastore.EventInterval{
				{Data: datastore.EventIntervalData{Interval: int64(44), Time: "2021-11"}, Count: uint64(2)},
				{Data: datastore.EventIntervalData{Interval: int64(48), Time: "2021-12"}, Count: uint64(1)},
				{Data: datastore.EventIntervalData{Interval: int64(52), Time: "2022-01"}, Count: uint64(4)},
			},
		},
		{
			name: "Load Event Intervals - Yearly",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			times: []string{
				"2021-11-01T20:01:20",
				"2021-11-01T11:01:20",
				"2021-12-12T13:01:20",
				"2022-01-01T00:01:20",
				"2022-01-01T00:01:20",
				"2022-01-12T01:01:20",
				"2022-01-12T00:01:20",
			},
			params: Params{start: "2021-11-01T00:00:00", end: "2022-02-01T00:00:00"},
			period: datastore.Yearly,
			expected: []datastore.EventInterval{
				{Data: datastore.EventIntervalData{Interval: int64(2021), Time: "2021"}, Count: uint64(3)},
				{Data: datastore.EventIntervalData{Interval: int64(2022), Time: "2022"}, Count: uint64(4)},
			},
		},
		{
			name: "Load Event Intervals - No End Date",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			times: []string{
				"2021-11-01T20:01:20",
				"2021-11-01T11:01:20",
				"2021-12-12T13:01:20",
				"2022-01-01T00:01:20",
				"2022-01-01T00:01:20",
				"2022-01-12T01:01:20",
				"2022-01-12T00:01:20",
			},
			params: Params{start: "2021-11-01T00:00:00"},
			period: datastore.Daily,
			expected: []datastore.EventInterval{
				{Data: datastore.EventIntervalData{Interval: int64(1), Time: "2021-11-01"}, Count: uint64(2)},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			eventRepo := NewEventRepo(db)
			groupRepo := NewGroupRepo(db)
			appRepo := NewApplicationRepo(db)

			app := &datastore.Application{
				Title:   "My Application",
				GroupID: tc.group.UID,
				UID:     uuid.NewString(),
			}

			// create the group and group application
			require.NoError(t, groupRepo.CreateGroup(context.Background(),
				&datastore.Group{UID: tc.group.UID, Name: tc.group.Name}))

			require.NoError(t, appRepo.CreateApplication(context.Background(), app))

			for _, tt := range tc.times {
				createdAt, err := time.Parse(timeFormat, tt)

				require.NoError(t, err)

				event := &datastore.Event{
					UID:              uuid.NewString(),
					EventType:        "king.taker",
					MatchedEndpoints: 1,
					Data:             []byte("{\"key\":\"value\"}"),
					CreatedAt:        primitive.NewDateTimeFromTime(createdAt),
					DocumentStatus:   datastore.ActiveDocumentStatus,
					AppMetadata: &datastore.AppMetadata{
						Title:        app.Title,
						UID:          app.UID,
						GroupID:      app.GroupID,
						SupportEmail: app.SupportEmail,
					},
				}
				require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
			}

			startDate, err := time.Parse(timeFormat, tc.params.start)
			require.NoError(t, err)

			var endDate time.Time
			if !util.IsStringEmpty(tc.params.end) {
				endDate, err = time.Parse(timeFormat, tc.params.end)
				require.NoError(t, err)
			}

			intervals, err := eventRepo.LoadEventIntervals(context.Background(),
				tc.group.UID, datastore.SearchParams{
					CreatedAtStart: startDate.Unix(),
					CreatedAtEnd:   endDate.Unix(),
				}, tc.period, 1)

			require.NoError(t, err)

			require.Equal(t, len(tc.expected), len(intervals))
			require.Equal(t, tc.expected, intervals)
		})
	}
}

func Test_LoadEventsPaged(t *testing.T) {
	type Group struct {
		UID  string
		Name string
	}

	type App struct {
		Title   string
		GroupID string
		UID     string
	}

	type Params struct {
		start string
		end   string
	}

	type Expected struct {
		EventCount     int
		paginationData datastore.PaginationData
	}

	tests := []struct {
		name     string
		group    Group
		app      App
		times    []string
		params   Params
		expected Expected
		pageData datastore.Pageable
	}{
		{
			name: "Load Event Paged - Start and End Date",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			app: App{
				Title:   "Application 1",
				GroupID: "gid-1",
				UID:     "aid-1",
			},
			times: []string{
				"2021-11-01T00:01:20",
				"2021-11-11T00:01:20",
				"2021-11-12T00:01:20",
				"2021-12-12T00:01:20",
				"2022-01-01T00:01:20",
				"2022-01-02T00:01:20",
				"2022-01-12T00:01:20",
				"2022-02-06T00:01:20",
				"2022-02-01T00:01:20",
				"2022-02-12T00:01:20",
			},
			params:   Params{start: "2021-11-01T00:00:00", end: "2022-02-01T00:00:00"},
			pageData: datastore.Pageable{Page: 1, PerPage: 3},
			expected: Expected{
				EventCount:     3,
				paginationData: datastore.PaginationData{Total: 7, TotalPage: 3, Page: 1, PerPage: 3, Prev: 0, Next: 2},
			},
		},
		{
			name: "Load Event Paged - Start Date Only",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			app: App{
				Title:   "Application 1",
				GroupID: "gid-1",
				UID:     "aid-1",
			},
			times: []string{
				"2021-11-01T00:01:20",
				"2021-11-11T00:01:20",
				"2021-11-12T00:01:20",
				"2021-12-12T00:01:20",
				"2022-01-01T00:01:20",
				"2022-01-02T00:01:20",
				"2022-01-12T00:01:20",
				"2022-02-01T00:01:20",
				"2022-02-06T00:01:20",
				"2022-02-12T00:01:20",
			},
			params:   Params{start: "2022-01-01T00:00:00"},
			pageData: datastore.Pageable{Page: 1, PerPage: 10},
			expected: Expected{
				EventCount:     6,
				paginationData: datastore.PaginationData{Total: 6, TotalPage: 1, Page: 1, PerPage: 10, Prev: 0, Next: 2},
			},
		},
		{
			name: "Load Event Paged - End Date Only",
			group: Group{
				UID:  "gid-1",
				Name: "Group 1",
			},
			app: App{
				Title:   "Application 1",
				GroupID: "gid-1",
				UID:     "aid-1",
			},
			times: []string{
				"2021-11-01T00:01:20",
				"2021-11-11T00:01:20",
				"2021-11-12T00:01:20",
				"2021-12-12T00:01:20",
				"2022-01-01T00:01:20",
				"2022-01-02T00:01:20",
				"2022-01-12T00:01:20",
				"2022-02-01T00:01:20",
				"2022-02-06T00:01:20",
				"2022-02-12T00:01:20",
			},
			params:   Params{end: "2022-01-01T00:00:00"},
			pageData: datastore.Pageable{Page: 1, PerPage: 5},
			expected: Expected{
				EventCount:     4,
				paginationData: datastore.PaginationData{Total: 4, TotalPage: 1, Page: 1, PerPage: 5, Prev: 0, Next: 2},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, closeFn := getDB(t)
			defer closeFn()

			eventRepo := NewEventRepo(db)
			groupRepo := NewGroupRepo(db)
			appRepo := NewApplicationRepo(db)

			app := &datastore.Application{
				Title:   tc.app.Title,
				GroupID: tc.app.GroupID,
				UID:     tc.app.UID,
			}

			// create the group and group application
			require.NoError(t, groupRepo.CreateGroup(context.Background(), &datastore.Group{
				UID:  tc.group.UID,
				Name: tc.group.Name,
			}))

			require.NoError(t, appRepo.CreateApplication(context.Background(), app))

			for _, tt := range tc.times {
				createdAt, err := time.Parse(timeFormat, tt)

				require.NoError(t, err)

				event := &datastore.Event{
					UID:              uuid.NewString(),
					EventType:        "king.taker",
					MatchedEndpoints: 1,
					Data:             []byte("{\"key\":\"value\"}"),
					CreatedAt:        primitive.NewDateTimeFromTime(createdAt),
					DocumentStatus:   datastore.ActiveDocumentStatus,
					AppMetadata: &datastore.AppMetadata{
						Title:        app.Title,
						UID:          app.UID,
						GroupID:      app.GroupID,
						SupportEmail: app.SupportEmail,
					},
				}
				require.NoError(t, eventRepo.CreateEvent(context.Background(), event))
			}

			var err error
			var endDate time.Time
			var startDate time.Time

			if !util.IsStringEmpty(tc.params.start) {
				startDate, err = time.Parse(timeFormat, tc.params.start)
				require.NoError(t, err)
			} else {
				startDate = time.Unix(0, 0)
			}

			if !util.IsStringEmpty(tc.params.end) {
				endDate, err = time.Parse(timeFormat, tc.params.end)
				require.NoError(t, err)
			} else {
				endDate = time.Unix(0, 0)
			}

			events, data, err := eventRepo.LoadEventsPaged(context.Background(), tc.group.UID, tc.app.UID, datastore.SearchParams{
				CreatedAtStart: startDate.Unix(),
				CreatedAtEnd:   endDate.Unix(),
			}, tc.pageData)

			require.NoError(t, err)

			require.Equal(t, tc.expected.EventCount, len(events))

			require.Equal(t, tc.expected.paginationData.Total, data.Total)
			require.Equal(t, tc.expected.paginationData.TotalPage, data.TotalPage)

			require.Equal(t, tc.expected.paginationData.Next, data.Next)
			require.Equal(t, tc.expected.paginationData.Prev, data.Prev)

			require.Equal(t, tc.expected.paginationData.Page, data.Page)
			require.Equal(t, tc.expected.paginationData.PerPage, data.PerPage)
		})
	}
}
