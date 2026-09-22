// Package inventory inspects queue stores without constructing writers,
// consumers or schedulers. A snapshot is never a drain certificate.
package inventory

import (
	"context"
	"errors"
	"time"
)

type Counts struct {
	Pending     int64 `json:"pending" db:"pending"`
	Processing  int64 `json:"processing" db:"processing"`
	Scheduled   int64 `json:"scheduled" db:"scheduled"`
	Retry       int64 `json:"retry" db:"retry"`
	Future      int64 `json:"future" db:"future"`
	Aggregating int64 `json:"aggregating" db:"aggregating"`
	Archived    int64 `json:"archived" db:"archived"`
	Completed   int64 `json:"completed" db:"completed"`
	Unknown     int64 `json:"unknown" db:"unknown"`
}

func (c Counts) remaining() bool {
	return c.Pending > 0 || c.Processing > 0 || c.Scheduled > 0 || c.Retry > 0 || c.Future > 0 || c.Aggregating > 0
}

type Queue struct {
	Name string `json:"name" db:"name"`
	Counts
	Paused         bool  `json:"paused" db:"paused"`
	OldestDueAgeMS int64 `json:"oldest_due_age_ms" db:"oldest_due_age_ms"`
}

type Snapshot struct {
	ObservedAt time.Time `json:"observed_at"`
	Queues     []Queue   `json:"queues"`
}

type Reader interface {
	Inspect(context.Context) (Snapshot, error)
}

type Store struct {
	ID                string    `json:"id"`
	Provider          string    `json:"provider"`
	Role              string    `json:"role"`
	Connection        string    `json:"connection"`
	Snapshot          *Snapshot `json:"snapshot"`
	Visible           bool      `json:"visible"`
	VisibilityReasons []string  `json:"visibility_reasons"`
	// Empty until the source fence, ownership and execution contract exist.
	Actions []string `json:"actions"`
}

type Registration struct {
	ID       string
	Provider string
	Role     string
	Reader   Reader
}

type Inventory struct{ stores []Registration }

func New(stores []Registration) (*Inventory, error) {
	seen := make(map[string]bool)
	active := 0
	for _, s := range stores {
		if s.ID == "" || seen[s.ID] {
			return nil, errors.New("queue store identity must be unique and nonempty")
		}
		if s.Provider != "redis" && s.Provider != "postgres" {
			return nil, errors.New("unsupported queue store provider")
		}
		if s.Role != "active" && s.Role != "previous" {
			return nil, errors.New("unsupported queue store role")
		}
		if s.Reader == nil {
			return nil, errors.New("queue store reader is required")
		}
		if s.Role == "active" {
			active++
		}
		seen[s.ID] = true
	}
	if active != 1 {
		return nil, errors.New("exactly one active queue store is required")
	}
	return &Inventory{stores: append([]Registration(nil), stores...)}, nil
}

// Inspect reports failures per store. In particular, a previous store remains
// visible on its first failed read, including after a browser/API restart.
// Errors may carry credentials and are deliberately not returned in the report.
func (i *Inventory) Inspect(ctx context.Context) []Store {
	result := make([]Store, 0, len(i.stores))
	for _, s := range i.stores {
		row := Store{ID: s.ID, Provider: s.Provider, Role: s.Role, Connection: "unknown", Actions: []string{}, VisibilityReasons: []string{}}
		storeCtx, cancel := context.WithTimeout(ctx, inspectionTimeout)
		snapshot, err := s.Reader.Inspect(storeCtx)
		if storeCtx.Err() != nil {
			err = storeCtx.Err()
		}
		cancel()
		if err != nil || snapshot.ObservedAt.IsZero() {
			row.VisibilityReasons = append(row.VisibilityReasons, "inspection_failed")
		} else {
			row.Connection = "connected"
			if snapshot.Queues == nil {
				snapshot.Queues = []Queue{}
			}
			row.Snapshot = &snapshot
			remaining, archived, unknown := false, false, false
			for _, q := range snapshot.Queues {
				remaining = remaining || q.Counts.remaining()
				archived = archived || q.Archived > 0
				unknown = unknown || q.Unknown > 0
			}
			if remaining {
				row.VisibilityReasons = append(row.VisibilityReasons, "remaining_work")
			}
			if archived {
				row.VisibilityReasons = append(row.VisibilityReasons, "archived_work")
			}
			if unknown {
				row.VisibilityReasons = append(row.VisibilityReasons, "unknown_work")
			}
		}
		row.Visible = s.Role == "active" || len(row.VisibilityReasons) > 0
		result = append(result, row)
	}
	return result
}
