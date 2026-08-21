package attach

import (
	"strings"
	"time"
)

const (
	PremakeDays = 10
	// CutoffDays is how far ahead of UTC midnight the history CHECK reaches.
	// A NOT VALID CHECK still applies to rows written after it is added, so
	// this has to outlive VALIDATE CONSTRAINT on a large adopted heap.
	CutoffDays  = 14
	lockTimeout = "3s"

	// The names a conversion parks a table's rows under while the live name
	// does not hold all of them.
	partitionedSuffix = "_partitioned" // detach: the parent, until drain finishes
	copyNewSuffix     = "_new"         // copy: the rebuilt table, before it takes the live name
	copyOldSuffix     = "_old"         // copy: the original, until it is dropped
)

// Leftovers are the relations that exist beside table only while a conversion
// has the rows split between them, or after one died with them still split.
//
// Detach renames the partitioned parent to <table>_partitioned and copies the
// rows back under the live name on a later statement, so the live name is
// short of every row written since the conversion for the length of that
// drain. The copy rewrite fills <table>_new, renames the original to
// <table>_old and the copy into place; that runs as one statement and so is
// not observable midway, but the conversion preflight treats a surviving
// <table>_new as a conversion that died partway, and this agrees with it.
//
// Anything reading a table to decide whether a row is gone has to consult this
// first: while one of these exists, absence from the live name is not a
// verdict.
func Leftovers(table string) []string {
	return []string{
		table + partitionedSuffix,
		table + copyNewSuffix,
		table + copyOldSuffix,
	}
}

// Spec is what differs between the four tables.
type Spec struct {
	Table             string
	ParentForeignKeys string
	ExtraNotNull      []string
	ValidateHint      string

	Prepare      []string
	Swap         []string // after rename, before the parent exists
	SwapEnd      []string // after ATTACH, still in the exclusive tx; SQL may name Table
	AfterAttach  []string
	DuringDetach []string // stand-in triggers on the live name, before drain
	AfterDetach  []string

	// CopyUnpartition is the old rewrite, used only when the table was converted
	// by copying and has no adopted heap to rename back.
	CopyUnpartition string
}

// Cutoff is the UTC instant that divides adopted history from partitioned
// future. Forward partition DATE math must use this instant in UTC, not the
// session time zone: `'…'::TIMESTAMPTZ::DATE` follows TimeZone and will not
// line up with the CHECK. A NOT VALID CHECK still applies to rows written
// after it is added, so the bound must outlive VALIDATE on a large table.
func Cutoff(now time.Time) time.Time {
	return now.UTC().Truncate(24*time.Hour).AddDate(0, 0, CutoffDays)
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (s Spec) defaultName() string { return s.Table + "_default" }
func (s Spec) pkIndex() string     { return s.Table + "_pk_part" }
func (s Spec) idIndex() string     { return s.Table + "_id_key" }
func (s Spec) bounds() string      { return s.Table + "_default_bounds" }
func (s Spec) partitioned() string { return s.Table + partitionedSuffix }
