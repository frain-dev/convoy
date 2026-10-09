package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/events/repo"
)

type existsPagingRecorder struct {
	last     string
	lastBody []byte
}

func (r *existsPagingRecorder) LoadEventsPagedExistsInnerDesc(ctx context.Context, arg repo.LoadEventsPagedExistsInnerDescParams) ([]repo.LoadEventsPagedExistsInnerDescRow, error) {
	r.last = "desc"
	r.lastBody = append([]byte(nil), arg.Body...)
	return nil, nil
}

func (r *existsPagingRecorder) LoadEventsPagedExistsInnerAsc(ctx context.Context, arg repo.LoadEventsPagedExistsInnerAscParams) ([]repo.LoadEventsPagedExistsInnerAscRow, error) {
	r.last = "asc"
	r.lastBody = append([]byte(nil), arg.Body...)
	return nil, nil
}

func (r *existsPagingRecorder) CopyRowsFromEventsToEventsSearch(context.Context, repo.CopyRowsFromEventsToEventsSearchParams) error {
	panic("unexpected CopyRowsFromEventsToEventsSearch")
}

func (r *existsPagingRecorder) CountEvents(context.Context, repo.CountEventsParams) (pgtype.Int8, error) {
	panic("unexpected CountEvents")
}

func (r *existsPagingRecorder) CountExportedEvents(context.Context, repo.CountExportedEventsParams) (pgtype.Int8, error) {
	panic("unexpected CountExportedEvents")
}

func (r *existsPagingRecorder) CountPrevEvents(context.Context, repo.CountPrevEventsParams) (pgtype.Int8, error) {
	panic("unexpected CountPrevEvents")
}

func (r *existsPagingRecorder) CountPrevEventsSearch(context.Context, repo.CountPrevEventsSearchParams) (pgtype.Int8, error) {
	panic("unexpected CountPrevEventsSearch")
}

func (r *existsPagingRecorder) CountProjectMessages(context.Context, pgtype.Text) (pgtype.Int8, error) {
	panic("unexpected CountProjectMessages")
}

func (r *existsPagingRecorder) CreateEvent(context.Context, repo.CreateEventParams) error {
	panic("unexpected CreateEvent")
}

func (r *existsPagingRecorder) CreateEventEndpoint(context.Context, []repo.CreateEventEndpointParams) *repo.CreateEventEndpointBatchResults {
	panic("unexpected CreateEventEndpoint")
}

func (r *existsPagingRecorder) ExportEvents(context.Context, repo.ExportEventsParams) ([]repo.ExportEventsRow, error) {
	panic("unexpected ExportEvents")
}

func (r *existsPagingRecorder) FindEventByID(context.Context, repo.FindEventByIDParams) (repo.FindEventByIDRow, error) {
	panic("unexpected FindEventByID")
}

func (r *existsPagingRecorder) FindEventsByIDs(context.Context, repo.FindEventsByIDsParams) ([]repo.FindEventsByIDsRow, error) {
	panic("unexpected FindEventsByIDs")
}

func (r *existsPagingRecorder) FindEventsByIdempotencyKey(context.Context, repo.FindEventsByIdempotencyKeyParams) (bool, error) {
	panic("unexpected FindEventsByIdempotencyKey")
}

func (r *existsPagingRecorder) FindFirstEventWithIdempotencyKey(context.Context, repo.FindFirstEventWithIdempotencyKeyParams) (repo.FindFirstEventWithIdempotencyKeyRow, error) {
	panic("unexpected FindFirstEventWithIdempotencyKey")
}

func (r *existsPagingRecorder) HardDeleteTokenizedEvents(context.Context, repo.HardDeleteTokenizedEventsParams) error {
	panic("unexpected HardDeleteTokenizedEvents")
}

func (r *existsPagingRecorder) LoadEventsPagedSearch(context.Context, repo.LoadEventsPagedSearchParams) ([]repo.LoadEventsPagedSearchRow, error) {
	panic("unexpected LoadEventsPagedSearch")
}

func (r *existsPagingRecorder) UpdateEventEndpoints(context.Context, repo.UpdateEventEndpointsParams) error {
	panic("unexpected UpdateEventEndpoints")
}

func (r *existsPagingRecorder) UpdateEventStatus(context.Context, repo.UpdateEventStatusParams) error {
	panic("unexpected UpdateEventStatus")
}

var _ repo.Querier = (*existsPagingRecorder)(nil)

func TestLoadEventsPagedExistsRowsRouting(t *testing.T) {
	cases := []struct {
		sort, direction, want string
	}{
		{sort: "DESC", direction: "next", want: "desc"},
		{sort: "ASC", direction: "prev", want: "desc"},
		{sort: "ASC", direction: "next", want: "asc"},
		{sort: "DESC", direction: "prev", want: "asc"},
	}

	for _, tc := range cases {
		t.Run(tc.sort+"_"+tc.direction, func(t *testing.T) {
			rec := &existsPagingRecorder{}
			svc := &Service{repo: rec}
			params := existsPagedQueryBase{
				SortOrder: pgtype.Text{String: tc.sort, Valid: true},
				Direction: pgtype.Text{String: tc.direction, Valid: true},
			}

			_, err := svc.loadEventsPagedExistsRows(context.Background(), params)
			require.NoError(t, err)
			require.Equal(t, tc.want, rec.last)
		})
	}
}

func TestLoadEventsPagedPreservesBodyFilterOnPagination(t *testing.T) {
	rec := &existsPagingRecorder{}
	svc := &Service{repo: rec}
	body := json.RawMessage(`{"status":"paid"}`)
	filter := &datastore.Filter{
		Body:                body,
		EventSearchLicensed: true,
		Project:             &datastore.Project{UID: "proj"},
		Pageable: datastore.Pageable{
			PerPage:    10,
			NextCursor: "cursor-1",
			Direction:  datastore.Next,
			Sort:       "DESC",
		},
		SearchParams: datastore.SearchParams{
			CreatedAtStart: time.Now().Add(-24 * time.Hour).Unix(),
			CreatedAtEnd:   time.Now().Unix(),
		},
	}

	_, _, err := svc.LoadEventsPaged(context.Background(), "proj", filter)
	require.NoError(t, err)
	require.Equal(t, "desc", rec.last)
	require.JSONEq(t, string(body), string(rec.lastBody))
}
