package events

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/events/repo"
)

type existsPagingRecorder struct {
	last string
}

func (r *existsPagingRecorder) LoadEventsPagedExistsInnerDesc(ctx context.Context, arg repo.LoadEventsPagedExistsParams) ([]repo.LoadEventsPagedExistsRow, error) {
	r.last = "desc"
	return nil, nil
}

func (r *existsPagingRecorder) LoadEventsPagedExistsInnerAsc(ctx context.Context, arg repo.LoadEventsPagedExistsParams) ([]repo.LoadEventsPagedExistsRow, error) {
	r.last = "asc"
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
			params := repo.LoadEventsPagedExistsParams{
				SortOrder: pgtype.Text{String: tc.sort, Valid: true},
				Direction: pgtype.Text{String: tc.direction, Valid: true},
			}

			_, err := svc.loadEventsPagedExistsRows(context.Background(), params)
			require.NoError(t, err)
			require.Equal(t, tc.want, rec.last)
		})
	}
}
