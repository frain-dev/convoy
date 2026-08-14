package attach

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDropConstraintSQLIncludesAdoptedChild(t *testing.T) {
	got := DropConstraintSQL("event_deliveries", "event_deliveries_event_id_fkey")
	require.Equal(t, []string{
		`ALTER TABLE convoy."event_deliveries" DROP CONSTRAINT IF EXISTS "event_deliveries_event_id_fkey"`,
		`ALTER TABLE IF EXISTS convoy."event_deliveries_default" DROP CONSTRAINT IF EXISTS "event_deliveries_event_id_fkey"`,
	}, got)
}

func TestDropExistingPrimaryKeySQLLooksUpTheConstraint(t *testing.T) {
	got := dropExistingPrimaryKeySQL("events_search_default")
	require.Contains(t, got, `c.relname = 'events_search_default'`)
	require.Contains(t, got, `con.contype = 'p'`)
	require.Contains(t, got, `ALTER TABLE convoy."events_search_default" DROP CONSTRAINT %I`)
	require.NotContains(t, got, `events_search_pkey`,
		"the leftover copy-unpartition name is {table}_new_pkey; do not hardcode {table}_pkey")
}

func TestCutoffIsUTCMidnightFourteenDaysOut(t *testing.T) {
	now := time.Date(2026, 8, 15, 3, 4, 5, 0, time.FixedZone("behind", -4*3600))
	got := Cutoff(now)
	require.True(t, got.Equal(time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)))
}
