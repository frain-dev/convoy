package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubMutator struct {
	calls   []string
	results map[string]error
}

func (s *stubMutator) run(action, taskID string) error {
	s.calls = append(s.calls, action+":"+taskID)
	return s.results[taskID]
}

func (s *stubMutator) RetryTask(_ context.Context, _, taskID string) error {
	return s.run(ActionRetry, taskID)
}

func (s *stubMutator) RunTask(_ context.Context, _, taskID string) error {
	return s.run(ActionRun, taskID)
}

func (s *stubMutator) ArchiveTask(_ context.Context, _, taskID string) error {
	return s.run(ActionArchive, taskID)
}

func (s *stubMutator) DeleteTask(_ context.Context, _, taskID string) error {
	return s.run(ActionDelete, taskID)
}

// The operator selected these rows together, so a task that moved out from
// under the selection is expected: the rest still run, and the refused one is
// named rather than folded into a count.
func TestRunBulkActionRecordsRefusals(t *testing.T) {
	m := &stubMutator{results: map[string]error{
		"gone":    ErrTaskNotFound,
		"claimed": ErrTaskStatusConflict,
	}}

	result, err := RunBulkAction(t.Context(), m, "EventQueue", ActionArchive, []string{"a", "gone", "b", "claimed"})
	require.NoError(t, err)
	require.Equal(t, 2, result.Succeeded)
	require.Len(t, result.Failures, 2)
	require.Contains(t, result.Failures["gone"], "not found")
	require.Equal(t, []string{"archive:a", "archive:gone", "archive:b", "archive:claimed"}, m.calls)
}

// A transport failure says nothing about the remaining ids, so it aborts rather
// than being recorded against each one as though the broker had rejected them.
func TestRunBulkActionAbortsOnTransportFailure(t *testing.T) {
	boom := errors.New("connection refused")
	m := &stubMutator{results: map[string]error{"b": boom}}

	result, err := RunBulkAction(t.Context(), m, "EventQueue", ActionDelete, []string{"a", "b", "c"})
	require.ErrorIs(t, err, boom)
	require.Equal(t, 1, result.Succeeded, "outcomes so far are still reported")
	require.Empty(t, result.Failures)
	require.Equal(t, []string{"delete:a", "delete:b"}, m.calls, "the remaining ids are not attempted")
}

func TestRunBulkActionRejectsBadInput(t *testing.T) {
	m := &stubMutator{}
	ctx := t.Context()

	_, err := RunBulkAction(ctx, m, "", ActionRetry, []string{"a"})
	require.ErrorIs(t, err, ErrQueueRequired)

	_, err = RunBulkAction(ctx, m, "EventQueue", ActionRetry, nil)
	require.ErrorIs(t, err, ErrNoTaskIDs)

	// A selection is made from a page, so it is bounded by one.
	tooMany := make([]string, MaxTasksPerPage+1)
	_, err = RunBulkAction(ctx, m, "EventQueue", ActionRetry, tooMany)
	require.ErrorIs(t, err, ErrTooManyTaskIDs)

	// An action no provider implements is rejected rather than reported as
	// zero rows moved, which would read as a successful no-op.
	_, err = RunBulkAction(ctx, m, "EventQueue", "explode", []string{"a"})
	require.ErrorIs(t, err, ErrUnknownTaskAction)

	require.Empty(t, m.calls)
}

// Every provider reads the page size through Size, so a caller that omits it
// and one that sends something absurd land on the same rules.
func TestTaskFilterSize(t *testing.T) {
	require.Equal(t, TasksPerPage, TaskFilter{}.Size())
	require.Equal(t, TasksPerPage, TaskFilter{PageSize: -5}.Size())
	require.Equal(t, 10, TaskFilter{PageSize: 10}.Size())
	require.Equal(t, MaxTasksPerPage, TaskFilter{PageSize: MaxTasksPerPage + 1}.Size())
}
