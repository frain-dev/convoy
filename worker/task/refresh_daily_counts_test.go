package task

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSkipIfLockBusy(t *testing.T) {
	require.NoError(t, skipIfLockBusy(ErrLockBusy))
	require.NoError(t, skipIfLockBusy(errors.Join(ErrLockBusy, errors.New("held"))))

	err := errors.New("write failed")
	require.ErrorIs(t, skipIfLockBusy(err), err)
	require.NoError(t, skipIfLockBusy(nil))
}
