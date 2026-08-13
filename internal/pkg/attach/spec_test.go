package attach

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCutoffIsUTCMidnightTwoDaysOut(t *testing.T) {
	now := time.Date(2026, 8, 15, 3, 4, 5, 0, time.FixedZone("behind", -4*3600))
	got := Cutoff(now)
	require.True(t, got.Equal(time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)))
}
