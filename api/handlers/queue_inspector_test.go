package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

type fakeInspector struct {
	err   error
	stats queue.Stats
	page  queue.TaskPage

	filter   queue.TaskFilter
	retried  [2]string
	archived [2]string
}

func (f *fakeInspector) Stats(context.Context) (queue.Stats, error) {
	return f.stats, f.err
}

func (f *fakeInspector) Tasks(_ context.Context, filter queue.TaskFilter) (queue.TaskPage, error) {
	f.filter = filter
	return f.page, f.err
}

func (f *fakeInspector) RetryTask(_ context.Context, queueName, taskID string) error {
	f.retried = [2]string{queueName, taskID}
	return f.err
}

func (f *fakeInspector) ArchiveTask(_ context.Context, queueName, taskID string) error {
	f.archived = [2]string{queueName, taskID}
	return f.err
}

func queueRequest(target string, params map[string]string, user *datastore.User) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, nil)

	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	if user != nil {
		ctx = context.WithValue(ctx, convoy.AuthUserCtx, &auth.AuthenticatedUser{User: user})
	}
	return req.WithContext(ctx)
}

func queueTaskRequest(taskID string, user *datastore.User) *http.Request {
	return queueRequest("/ui/admin/queue/EventQueue/tasks/"+taskID+"/retry",
		map[string]string{"queueName": "EventQueue", "taskID": taskID}, user)
}

func newQueueHandler(t *testing.T, inspector queue.Inspector, expectRepo func(*mocks.MockOrganisationMemberRepository)) *Handler {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mocks.NewMockOrganisationMemberRepository(ctrl)
	if expectRepo != nil {
		expectRepo(repo)
	}
	return &Handler{A: &types.APIOptions{
		QueueInspector: inspector,
		OrgMemberRepo:  repo,
		Logger:         log.New("convoy", log.LevelInfo),
	}}
}

func adminOnce(times int) func(*mocks.MockOrganisationMemberRepository) {
	return func(repo *mocks.MockOrganisationMemberRepository) {
		repo.EXPECT().HasInstanceAdminAccess(gomock.Any(), "user-1").Return(true, nil).Times(times)
	}
}

// Queue contents span every org on the instance, so each endpoint answers to
// the instance-admin policy on its own rather than trusting a route group.
func TestQueueEndpointsRequireInstanceAdmin(t *testing.T) {
	user := &datastore.User{UID: "user-1"}

	principals := []struct {
		name       string
		user       *datastore.User
		expectRepo func(*mocks.MockOrganisationMemberRepository)
		wantStatus int
	}{
		{
			// An API key authenticates but carries no user, so it lands here
			// too: queue monitoring is for operators, not for integrations.
			name:       "unauthenticated caller",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "authenticated non-admin",
			user: user,
			expectRepo: func(repo *mocks.MockOrganisationMemberRepository) {
				repo.EXPECT().HasInstanceAdminAccess(gomock.Any(), "user-1").Return(false, nil)
			},
			wantStatus: http.StatusForbidden,
		},
		{
			// Fail closed: an unreadable membership is not an admin grant.
			name: "membership lookup fails",
			user: user,
			expectRepo: func(repo *mocks.MockOrganisationMemberRepository) {
				repo.EXPECT().HasInstanceAdminAccess(gomock.Any(), "user-1").Return(false, fmt.Errorf("db down"))
			},
			wantStatus: http.StatusForbidden,
		},
	}

	endpoints := map[string]func(*Handler) http.HandlerFunc{
		"stats":   func(h *Handler) http.HandlerFunc { return h.GetQueueStats },
		"tasks":   func(h *Handler) http.HandlerFunc { return h.GetQueueTasks },
		"retry":   func(h *Handler) http.HandlerFunc { return h.RetryQueueTask },
		"archive": func(h *Handler) http.HandlerFunc { return h.ArchiveQueueTask },
	}

	for name, endpoint := range endpoints {
		for _, tc := range principals {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				inspector := &fakeInspector{}
				h := newQueueHandler(t, inspector, tc.expectRepo)

				rec := httptest.NewRecorder()
				endpoint(h)(rec, queueTaskRequest("task-1", tc.user))

				require.Equal(t, tc.wantStatus, rec.Code)
				require.Empty(t, inspector.retried[1], "the action must not run for an unauthorized caller")
				require.Empty(t, inspector.archived[1], "the action must not run for an unauthorized caller")
				require.Empty(t, inspector.filter.Queue, "the read must not run for an unauthorized caller")
			})
		}
	}
}

func TestGetQueueStats(t *testing.T) {
	inspector := &fakeInspector{stats: queue.Stats{
		Provider: queue.ProviderPostgres,
		Statuses: []string{queue.StatusPending},
		Queues:   []queue.QueueStat{{Queue: "EventQueue", Counts: map[string]int64{queue.StatusPending: 3}}},
	}}
	h := newQueueHandler(t, inspector, adminOnce(1))

	rec := httptest.NewRecorder()
	h.GetQueueStats(rec, queueTaskRequest("", &datastore.User{UID: "user-1"}))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"provider":"postgres"`)
	require.Contains(t, rec.Body.String(), `"pending":3`)
}

func TestGetQueueTasksPassesFilter(t *testing.T) {
	inspector := &fakeInspector{page: queue.TaskPage{Page: 2}}
	h := newQueueHandler(t, inspector, adminOnce(1))

	req := queueRequest("/ui/admin/queue/EventQueue/tasks?status=archived&page=2",
		map[string]string{"queueName": "EventQueue"}, &datastore.User{UID: "user-1"})

	rec := httptest.NewRecorder()
	h.GetQueueTasks(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, queue.TaskFilter{Queue: "EventQueue", Status: queue.StatusArchived, Page: 2}, inspector.filter)
}

func TestGetQueueTasksRejectsNonNumericPage(t *testing.T) {
	inspector := &fakeInspector{}
	h := newQueueHandler(t, inspector, adminOnce(1))

	req := queueRequest("/ui/admin/queue/EventQueue/tasks?status=pending&page=abc",
		map[string]string{"queueName": "EventQueue"}, &datastore.User{UID: "user-1"})

	rec := httptest.NewRecorder()
	h.GetQueueTasks(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, inspector.filter.Queue)
}

func TestQueueTaskActionOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "success",
			wantStatus: http.StatusOK,
		},
		{
			name:       "task not found",
			err:        queue.ErrTaskNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrong status",
			err:        fmt.Errorf("%w: task is pending, want archived", queue.ErrTaskStatusConflict),
			wantStatus: http.StatusConflict,
		},
		{
			name:       "scheduler row",
			err:        queue.ErrCronTaskImmutable,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "unknown status",
			err:        fmt.Errorf("%w: %q", queue.ErrUnknownTaskStatus, "bogus"),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "broker error",
			err:        fmt.Errorf("pq: connection refused"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			inspector := &fakeInspector{err: tc.err}
			h := newQueueHandler(t, inspector, adminOnce(2))

			rec := httptest.NewRecorder()
			h.RetryQueueTask(rec, queueTaskRequest("task-1", &datastore.User{UID: "user-1"}))
			require.Equal(t, tc.wantStatus, rec.Code)
			require.Equal(t, [2]string{"EventQueue", "task-1"}, inspector.retried)

			rec = httptest.NewRecorder()
			h.ArchiveQueueTask(rec, queueTaskRequest("task-2", &datastore.User{UID: "user-1"}))
			require.Equal(t, tc.wantStatus, rec.Code)
			require.Equal(t, [2]string{"EventQueue", "task-2"}, inspector.archived)
		})
	}

	// The raw failure carries broker and database detail, so it stays in the log.
	t.Run("broker error is not echoed", func(t *testing.T) {
		inspector := &fakeInspector{err: fmt.Errorf("pq: password authentication failed for user \"convoy\"")}
		h := newQueueHandler(t, inspector, adminOnce(1))

		rec := httptest.NewRecorder()
		h.RetryQueueTask(rec, queueTaskRequest("task-1", &datastore.User{UID: "user-1"}))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "password authentication failed")
	})
}

// A deployment with no inspector wired must say so rather than panic or report
// success.
func TestQueueEndpointsWithoutInspector(t *testing.T) {
	h := newQueueHandler(t, nil, adminOnce(2))

	rec := httptest.NewRecorder()
	h.GetQueueStats(rec, queueTaskRequest("", &datastore.User{UID: "user-1"}))
	require.Equal(t, http.StatusNotImplemented, rec.Code)

	rec = httptest.NewRecorder()
	h.RetryQueueTask(rec, queueTaskRequest("task-1", &datastore.User{UID: "user-1"}))
	require.Equal(t, http.StatusNotImplemented, rec.Code)
}

// Asynqmon is reached by a browser navigation with no Authorization header, so
// its mount answers to the same policy through this middleware.
func TestRequireQueueMonitoringAdmin(t *testing.T) {
	tests := []struct {
		name       string
		user       *datastore.User
		expectRepo func(*mocks.MockOrganisationMemberRepository)
		wantStatus int
		wantServed bool
	}{
		{
			name:       "no user",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "non-admin",
			user: &datastore.User{UID: "user-1"},
			expectRepo: func(repo *mocks.MockOrganisationMemberRepository) {
				repo.EXPECT().HasInstanceAdminAccess(gomock.Any(), "user-1").Return(false, nil)
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "instance admin",
			user:       &datastore.User{UID: "user-1"},
			expectRepo: adminOnce(1),
			wantStatus: http.StatusOK,
			wantServed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newQueueHandler(t, &fakeInspector{}, tc.expectRepo)

			served := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				served = true
				_, _ = w.Write([]byte("queue contents"))
			})

			rec := httptest.NewRecorder()
			h.RequireQueueMonitoringAdmin()(next).ServeHTTP(rec, queueTaskRequest("task-1", tc.user))

			require.Equal(t, tc.wantStatus, rec.Code)
			require.Equal(t, tc.wantServed, served)
			if !tc.wantServed {
				require.NotContains(t, rec.Body.String(), "queue contents")
			}
		})
	}
}
