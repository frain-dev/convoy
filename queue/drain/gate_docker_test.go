//go:build integration

package drain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerAdmissionGate(t *testing.T) {
	r := dockerRepository(t)
	gate := &Gate{Repository: r, Scope: "gate", StoreID: "source"}
	active := &Gate{Repository: r, Scope: "gate", StoreID: "active"}
	var accepted context.Context
	entered, finish := make(chan struct{}), make(chan struct{})
	outcome := make(chan error, 1)
	go func() {
		outcome <- gate.Run(t.Context(), Producer, func(ctx context.Context) error {
			accepted = ctx
			close(entered)
			<-finish
			return gate.Run(ctx, Producer, func(context.Context) error { return nil })
		})
	}()
	<-entered
	op, err := r.Begin(t.Context(), Target{Scope: "gate", StoreID: "source", Provider: "redis", ConfigurationRevision: "cfg", Purpose: DrainPrevious}, "start")
	require.NoError(t, err)
	require.ErrorIs(t, gate.Run(t.Context(), Producer, func(context.Context) error { t.Error("fenced work ran"); return nil }), ErrFenced)
	require.NoError(t, active.Run(t.Context(), Producer, func(context.Context) error { return nil }))
	e := Evidence{Epoch: op.Epoch, ConfigurationRevision: op.ConfigurationRevision, AdmissionClosed: true, WritersFenced: true, InspectionSucceeded: true}
	_, err = r.Advance(t.Context(), op.Scope, op.ID, op.Revision, Draining, e)
	require.ErrorIs(t, err, ErrEvidence)
	close(finish)
	require.NoError(t, <-outcome)
	require.ErrorIs(t, gate.Run(accepted, Producer, func(context.Context) error { t.Error("late descendant ran"); return nil }), ErrFenced)
	op, err = r.Advance(t.Context(), op.Scope, op.ID, op.Revision, Draining, e)
	require.NoError(t, err)
	handlerErr := errors.New("rate limited")
	require.Same(t, handlerErr, gate.Run(t.Context(), Claim, func(ctx context.Context) error {
		require.Error(t, active.Run(ctx, Producer, func(context.Context) error { t.Error("changed store"); return nil }))
		return handlerErr
	}))
	var outstanding int
	require.NoError(t, r.db.GetContext(t.Context(), &outstanding, `SELECT count(*) FROM convoy.queue_operation_receipts WHERE scope='gate' AND settled_at IS NULL`))
	require.Zero(t, outstanding, "ordinary handler errors settle without changing their type")
	require.ErrorIs(t, gate.Run(t.Context(), Claim, func(ctx context.Context) error {
		_ = gate.Run(ctx, Producer, func(context.Context) error { return errors.New("ambiguous enqueue") })
		return nil
	}), ErrEvidence)
	require.NoError(t, r.db.GetContext(t.Context(), &outstanding, `SELECT count(*) FROM convoy.queue_operation_receipts WHERE scope='gate' AND settled_at IS NULL`))
	require.Equal(t, 1, outstanding, "ignored child failure cannot certify completion")
}
