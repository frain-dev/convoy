package broker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/internal/pkg/pglock"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/worker/task"
)

func newLockerWithMock(t *testing.T, maxConns int) (*postgresJobLocker, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return newPostgresJobLockerWithLimit(sqlx.NewDb(db, "sqlmock"), log.New("test", log.LevelError), maxConns), mock
}

func expectTryLock(mock sqlmock.Sqlmock, ok bool) {
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(ok))
}

func expectUnlock(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`SELECT pg_advisory_unlock\(\$1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"ok"}).AddRow(true))
}

func TestPostgresJobLockerReleasesOnSuccess(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	expectTryLock(mock, true)
	expectUnlock(mock)

	err := locker.WithLock(context.Background(), "convoy:test:success", time.Minute, func(context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresJobLockerReleasesOnError(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	expectTryLock(mock, true)
	expectUnlock(mock)

	boom := errors.New("boom")
	err := locker.WithLock(context.Background(), "convoy:test:error", time.Minute, func(context.Context) error {
		return boom
	})
	require.ErrorIs(t, err, boom)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresJobLockerPassesCancellableContextToCallback(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	expectTryLock(mock, true)
	expectUnlock(mock)

	var seen context.Context
	err := locker.WithLock(context.Background(), "convoy:test:ctx", time.Minute, func(runCtx context.Context) error {
		seen = runCtx
		deadline, ok := runCtx.Deadline()
		require.True(t, ok, "callback context must carry the max runtime deadline")
		require.WithinDuration(t, time.Now().Add(time.Minute), deadline, 5*time.Second)
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, seen)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresJobLockerDoesNotCancelHealthyLongCallback(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	expectTryLock(mock, true)
	expectUnlock(mock)

	// The smallest maxRuntime any caller now passes is minutes, so a callback
	// running well past the old one-second lease TTL must finish untouched.
	err := locker.WithLock(context.Background(), "convoy:test:healthy", 5*time.Minute, func(runCtx context.Context) error {
		time.Sleep(1100 * time.Millisecond)
		return runCtx.Err()
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunBoundedSharedByBothProviders(t *testing.T) {
	// runBounded is what the Redis provider runs fn through too, so the max
	// runtime contract is identical on both.
	err := runBounded(context.Background(), 5*time.Minute, func(runCtx context.Context) error {
		time.Sleep(1100 * time.Millisecond)
		return runCtx.Err()
	})
	require.NoError(t, err)

	err = runBounded(context.Background(), 20*time.Millisecond, func(runCtx context.Context) error {
		<-runCtx.Done()
		return nil
	})
	require.ErrorContains(t, err, "lock section exceeded max runtime")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPostgresJobLockerMutualExclusion(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	expectTryLock(mock, true)
	expectTryLock(mock, false)
	expectUnlock(mock)

	holding := make(chan struct{})
	release := make(chan struct{})
	var firstErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		firstErr = locker.WithLock(context.Background(), "convoy:test:exclusive", time.Minute, func(context.Context) error {
			close(holding)
			<-release
			return nil
		})
	}()

	<-holding
	secondErr := locker.WithLock(context.Background(), "convoy:test:exclusive", time.Minute, func(context.Context) error {
		t.Fatal("second holder must not run the critical section")
		return nil
	})
	require.ErrorIs(t, secondErr, task.ErrLockBusy)
	close(release)
	wg.Wait()
	require.NoError(t, firstErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresJobLockerMaxRuntimeCancelsCallbackWithoutReleasingLock(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	// Ordered expectations: the second acquire is refused (as Postgres does while
	// the lock is held) and the unlock only arrives afterwards. If the max runtime released
	// the lock, the unlock would come first and this ordering would fail.
	expectTryLock(mock, true)
	expectTryLock(mock, false)
	expectUnlock(mock)

	cancelled := make(chan struct{})
	finish := make(chan struct{})
	var firstErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		firstErr = locker.WithLock(context.Background(), "convoy:test:maxruntime", 30*time.Millisecond, func(runCtx context.Context) error {
			<-runCtx.Done()
			close(cancelled)
			<-finish
			return nil
		})
	}()

	<-cancelled

	// Expiry has fired and the callback is still running: the advisory lock is
	// still held, so a second holder must not be able to start the same job.
	secondErr := locker.WithLock(context.Background(), "convoy:test:maxruntime", time.Minute, func(context.Context) error {
		t.Fatal("second holder must not run while the first callback is still executing")
		return nil
	})
	require.ErrorIs(t, secondErr, task.ErrLockBusy)
	require.ErrorContains(t, secondErr, pglock.ErrNotObtained.Error())

	close(finish)
	wg.Wait()
	require.ErrorContains(t, firstErr, "lock section exceeded max runtime")
	require.ErrorIs(t, firstErr, context.DeadlineExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresJobLockerReleasesAfterCancelledCallbackReturns(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	expectTryLock(mock, true)
	expectUnlock(mock)
	expectTryLock(mock, true)
	expectUnlock(mock)

	err := locker.WithLock(context.Background(), "convoy:test:maxruntime:release", 20*time.Millisecond, func(runCtx context.Context) error {
		<-runCtx.Done()
		return nil
	})
	require.ErrorContains(t, err, "lock section exceeded max runtime")

	// The lock was released once the callback returned, so the next run acquires it.
	err = locker.WithLock(context.Background(), "convoy:test:maxruntime:release", time.Minute, func(context.Context) error {
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresJobLockerReleasesWhenCancelledCallbackErrors(t *testing.T) {
	locker, mock := newLockerWithMock(t, 4)
	expectTryLock(mock, true)
	expectUnlock(mock)

	err := locker.WithLock(context.Background(), "convoy:test:maxruntime:error", 20*time.Millisecond, func(runCtx context.Context) error {
		<-runCtx.Done()
		return runCtx.Err()
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresJobLockerBoundsConcurrentHolders(t *testing.T) {
	locker, _ := newLockerWithMock(t, 1)

	// Saturate the dedicated lock-holder limit without holding a real advisory lock.
	locker.slots <- struct{}{}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	err := locker.WithLock(ctx, "convoy:test:bound", time.Minute, func(context.Context) error {
		t.Fatal("must not enter critical section when lock connection slots are exhausted")
		return nil
	})
	require.ErrorContains(t, err, "failed to obtain lock")
	require.ErrorContains(t, err, "context deadline exceeded")
	require.Equal(t, 1, cap(locker.slots))
	require.Equal(t, 1, locker.db.Stats().MaxOpenConnections)
}

func TestOpenJobLockDBRequiresDSN(t *testing.T) {
	_, err := defaultOpenJobLockDB(config.DatabaseConfiguration{})
	require.EqualError(t, err, "postgres job locker requires a database dsn")
}

func TestPostgresJobLockerEnforcesMaxOpenConns(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	sqlxDB := sqlx.NewDb(db, "sqlmock")
	locker := newPostgresJobLockerWithLimit(sqlxDB, log.New("test", log.LevelError), 3)
	require.Equal(t, 3, locker.db.Stats().MaxOpenConnections)
	require.Equal(t, 3, cap(locker.slots))
}

func TestRunWithLeaseRenewalCancelsCallbackOnExtendFailure(t *testing.T) {
	logger := log.New("test", log.LevelError)
	cancelled := make(chan struct{})
	var extendCalls atomic.Int32

	err := runWithLeaseRenewal(
		context.Background(),
		5*time.Minute,
		20*time.Millisecond,
		func(context.Context) (bool, error) {
			extendCalls.Add(1)
			return false, errors.New("redis down")
		},
		logger,
		func(runCtx context.Context) error {
			<-runCtx.Done()
			close(cancelled)
			return nil
		},
	)

	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("callback context was not cancelled after extend failure")
	}
	require.ErrorIs(t, err, ErrLockLeaseLost)
	require.GreaterOrEqual(t, extendCalls.Load(), int32(1))
}

func TestRunWithLeaseRenewalHealthyRenewalLetsCallbackFinish(t *testing.T) {
	logger := log.New("test", log.LevelError)
	var extendCalls atomic.Int32

	err := runWithLeaseRenewal(
		context.Background(),
		5*time.Minute,
		20*time.Millisecond,
		func(context.Context) (bool, error) {
			extendCalls.Add(1)
			return true, nil
		},
		logger,
		func(runCtx context.Context) error {
			// Stay past at least one successful renew tick.
			time.Sleep(70 * time.Millisecond)
			return runCtx.Err()
		},
	)

	require.NoError(t, err)
	require.GreaterOrEqual(t, extendCalls.Load(), int32(1))
}

// TestPostgresJobLockerReportsBusyWhenSlotsSaturated covers a caller that could
// wait but should not. The slot is held for the holder's whole run, and those
// runs differ by orders of magnitude: a retention cron holds one for half an
// hour while a per-request billing recompute wants one for seconds. A caller
// that waits out a long holder ties up a goroutine and, worse, lets a burst of
// short callers stall the crons. Contention is reported instead so both skip.
func TestPostgresJobLockerReportsBusyWhenSlotsSaturated(t *testing.T) {
	original := slotWait
	slotWait = 20 * time.Millisecond
	t.Cleanup(func() { slotWait = original })

	locker, _ := newLockerWithMock(t, 1)
	locker.slots <- struct{}{}

	// Caller deadline far exceeds the slot wait, so the bound under test is the
	// slot wait and not the caller's context.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	err := locker.WithLock(ctx, "convoy:test:busy", time.Minute, func(context.Context) error {
		t.Fatal("must not enter the critical section without a slot")
		return nil
	})
	require.ErrorIs(t, err, task.ErrLockBusy)
	require.NoError(t, ctx.Err(), "the caller's context must be left usable")
}
