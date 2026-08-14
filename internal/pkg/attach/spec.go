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
)

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
func (s Spec) partitioned() string { return s.Table + "_partitioned" }
