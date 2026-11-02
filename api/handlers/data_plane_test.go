package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/types"
	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/dataplanestats"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type fakeDataPlaneStore struct {
	status     dataplanestats.Status
	err        error
	staleAfter time.Duration
	reads      int
}

func (f *fakeDataPlaneStore) PublishSnapshot(context.Context, dataplanestats.Snapshot) error {
	return f.err
}

func (f *fakeDataPlaneStore) ExpireSnapshots(context.Context, time.Duration) (int64, error) {
	return 0, f.err
}

func (f *fakeDataPlaneStore) DataPlaneStatus(_ context.Context, staleAfter time.Duration) (dataplanestats.Status, error) {
	f.staleAfter = staleAfter
	f.reads++

	return f.status, f.err
}

type fakeDataPlaneReporter struct {
	snapshot dataplanestats.Snapshot
	err      error
	reads    int
}

func (f *fakeDataPlaneReporter) Snapshot(context.Context) (dataplanestats.Snapshot, error) {
	f.reads++
	return f.snapshot, f.err
}

func dataPlaneRequest(target string, user *datastore.User) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)

	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, chi.NewRouteContext())
	if user != nil {
		ctx = context.WithValue(ctx, convoy.AuthUserCtx, &auth.AuthenticatedUser{User: user})
	}

	return req.WithContext(ctx)
}

func newDataPlaneHandler(t *testing.T, opts *types.APIOptions, expectRepo func(*mocks.MockOrganisationMemberRepository)) *Handler {
	t.Helper()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mocks.NewMockOrganisationMemberRepository(ctrl)
	if expectRepo != nil {
		expectRepo(repo)
	}

	opts.OrgMemberRepo = repo
	opts.Logger = log.New("convoy", log.LevelError)

	return &Handler{A: opts}
}

// Data plane depth spans every org on the instance, so both endpoints answer to
// the instance-admin policy on their own rather than trusting a route group.
func TestDataPlaneEndpointsRequireInstanceAdmin(t *testing.T) {
	user := &datastore.User{UID: "user-1"}

	principals := []struct {
		name       string
		user       *datastore.User
		expectRepo func(*mocks.MockOrganisationMemberRepository)
		wantStatus int
	}{
		{
			name:       "unauthenticated caller",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "authenticated non-admin",
			user: user,
			expectRepo: func(repo *mocks.MockOrganisationMemberRepository) {
				repo.EXPECT().FetchInstanceAdminByUserID(gomock.Any(), "user-1").Return(nil, datastore.ErrOrgMemberNotFound)
			},
			wantStatus: http.StatusForbidden,
		},
		{
			// Fail closed: an unreadable membership is not an admin grant.
			name: "membership lookup fails",
			user: user,
			expectRepo: func(repo *mocks.MockOrganisationMemberRepository) {
				repo.EXPECT().FetchInstanceAdminByUserID(gomock.Any(), "user-1").Return(nil, fmt.Errorf("db down"))
			},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range principals {
		t.Run("status/"+tc.name, func(t *testing.T) {
			store := &fakeDataPlaneStore{}
			h := newDataPlaneHandler(t, &types.APIOptions{DataPlaneMonitor: store}, tc.expectRepo)

			rec := httptest.NewRecorder()
			h.GetDataPlaneStatus(rec, dataPlaneRequest("/ui/admin/dataplane/status", tc.user))

			require.Equal(t, tc.wantStatus, rec.Code)
			require.Zero(t, store.reads, "the read must not run for an unauthorized caller")
		})

		t.Run("snapshot/"+tc.name, func(t *testing.T) {
			reporter := &fakeDataPlaneReporter{}
			h := newDataPlaneHandler(t, &types.APIOptions{DataPlaneReporter: reporter}, tc.expectRepo)

			rec := httptest.NewRecorder()
			h.GetDataPlaneSnapshot(rec, dataPlaneRequest("/ui/admin/dataplane/snapshot", tc.user))

			require.Equal(t, tc.wantStatus, rec.Code)
			require.Zero(t, reporter.reads, "the read must not run for an unauthorized caller")
		})
	}
}

// No provider is 501, not an empty report: an empty report is read as a plane
// holding no work, which is the misreading this surface exists to prevent.
func TestDataPlaneEndpointsAnswerNotImplementedWithoutAProvider(t *testing.T) {
	admin := func(repo *mocks.MockOrganisationMemberRepository) {
		repo.EXPECT().FetchInstanceAdminByUserID(gomock.Any(), "user-1").Return(&datastore.OrganisationMember{
			Role: auth.Role{Type: auth.RoleInstanceAdmin},
		}, nil)
	}

	statusHandler := newDataPlaneHandler(t, &types.APIOptions{}, admin)
	rec := httptest.NewRecorder()
	statusHandler.GetDataPlaneStatus(rec, dataPlaneRequest("/ui/admin/dataplane/status", &datastore.User{UID: "user-1"}))
	require.Equal(t, http.StatusNotImplemented, rec.Code)

	snapshotHandler := newDataPlaneHandler(t, &types.APIOptions{}, admin)
	rec = httptest.NewRecorder()
	snapshotHandler.GetDataPlaneSnapshot(rec, dataPlaneRequest("/ui/admin/dataplane/snapshot", &datastore.User{UID: "user-1"}))
	require.Equal(t, http.StatusNotImplemented, rec.Code)
}

func adminAdmits(times int) func(*mocks.MockOrganisationMemberRepository) {
	return func(repo *mocks.MockOrganisationMemberRepository) {
		repo.EXPECT().FetchInstanceAdminByUserID(gomock.Any(), "user-1").Return(&datastore.OrganisationMember{
			Role: auth.Role{Type: auth.RoleInstanceAdmin},
		}, nil).Times(times)
	}
}

// The staleness threshold is derived from the one interval that already owns
// sampling, and the server sends it so no client has to guess it.
func TestGetDataPlaneStatusDerivesStalenessFromTheSampleInterval(t *testing.T) {
	store := &fakeDataPlaneStore{status: dataplanestats.Status{
		Replicas: []dataplanestats.Replica{{
			Snapshot: dataplanestats.Snapshot{Replica: "pod-1", Mode: "example", Running: true},
			Stale:    true,
		}},
	}}

	opts := &types.APIOptions{DataPlaneMonitor: store}
	opts.Cfg.Metrics.Prometheus.SampleTime = 10
	h := newDataPlaneHandler(t, opts, adminAdmits(1))

	rec := httptest.NewRecorder()
	h.GetDataPlaneStatus(rec, dataPlaneRequest("/ui/admin/dataplane/status", &datastore.User{UID: "user-1"}))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 30*time.Second, store.staleAfter)
	require.Contains(t, rec.Body.String(), `"stale":true`)
	require.Contains(t, rec.Body.String(), `"pod-1"`)
}

// An unset sample time still has to produce a threshold, or every replica reads
// as stale the moment it publishes.
func TestGetDataPlaneStatusFallsBackToTheDefaultInterval(t *testing.T) {
	store := &fakeDataPlaneStore{}
	h := newDataPlaneHandler(t, &types.APIOptions{DataPlaneMonitor: store}, adminAdmits(1))

	rec := httptest.NewRecorder()
	h.GetDataPlaneStatus(rec, dataPlaneRequest("/ui/admin/dataplane/status", &datastore.User{UID: "user-1"}))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, dataplanestats.StaleAfter(dataplanestats.DefaultInterval), store.staleAfter)
}

// A backlog that could not be read is served as unknown, and the client sees
// that flag rather than a zero it would render as an empty queue.
func TestGetDataPlaneSnapshotServesUnknownBacklogsAsUnknown(t *testing.T) {
	reporter := &fakeDataPlaneReporter{snapshot: dataplanestats.Snapshot{
		Replica: "pod-1",
		Mode:    "example",
		Running: true,
		Outstanding: []dataplanestats.Backlog{
			{Name: "events_pending", Known: false},
			{Name: "deliveries_retry", Count: 7, Known: true},
		},
	}}
	h := newDataPlaneHandler(t, &types.APIOptions{DataPlaneReporter: reporter}, adminAdmits(1))

	rec := httptest.NewRecorder()
	h.GetDataPlaneSnapshot(rec, dataPlaneRequest("/ui/admin/dataplane/snapshot", &datastore.User{UID: "user-1"}))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `{"name":"events_pending","count":0,"known":false`)
	require.Contains(t, rec.Body.String(), `{"name":"deliveries_retry","count":7,"known":true`)
}

func TestDataPlaneReadFailuresAreServerErrors(t *testing.T) {
	store := &fakeDataPlaneStore{err: errors.New("db down")}
	h := newDataPlaneHandler(t, &types.APIOptions{DataPlaneMonitor: store}, adminAdmits(1))

	rec := httptest.NewRecorder()
	h.GetDataPlaneStatus(rec, dataPlaneRequest("/ui/admin/dataplane/status", &datastore.User{UID: "user-1"}))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	// The underlying error carries connection detail, so it belongs in the log
	// and not in the reply.
	require.NotContains(t, rec.Body.String(), "db down")

	reporter := &fakeDataPlaneReporter{err: errors.New("no plane")}
	h = newDataPlaneHandler(t, &types.APIOptions{DataPlaneReporter: reporter}, adminAdmits(1))

	rec = httptest.NewRecorder()
	h.GetDataPlaneSnapshot(rec, dataPlaneRequest("/ui/admin/dataplane/snapshot", &datastore.User{UID: "user-1"}))
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "no plane")
}
