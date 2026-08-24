package event_deliveries

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/frain-dev/convoy/datastore"
)

// FilterEventTypeLimit caps distinct observed types so a noisy project cannot
// dump an unbounded list into the Event Deliveries dropdown.
const FilterEventTypeLimit = 200

// CatalogFilterNames returns declared catalog names for the Event Deliveries
// filter. Wildcard "*", empty names, and deprecated types are omitted. Catalog
// rows themselves are unchanged: ingest does not write them.
func CatalogFilterNames(types []datastore.ProjectEventType) []string {
	return collectDeclaredNames(types, true)
}

// declaredFilterNames returns every declared name, including deprecated.
// Observed uses this as the exclusion set so a deprecated catalog type with
// traffic does not reappear as traffic-only.
func declaredFilterNames(types []datastore.ProjectEventType) []string {
	return collectDeclaredNames(types, false)
}

func collectDeclaredNames(types []datastore.ProjectEventType, dropDeprecated bool) []string {
	seen := make(map[string]struct{}, len(types))
	out := make([]string, 0, len(types))
	for _, et := range types {
		name := strings.TrimSpace(et.Name)
		if name == "" || name == "*" {
			continue
		}
		if dropDeprecated && et.DeprecatedAt.Valid {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// GroupFilterEventTypes splits names for the grouped dropdown. Catalog keeps
// non-deprecated declared names. Observed keeps traffic-only names that are
// not already declared, including names declared but deprecated. Both slices
// are sorted and never nil.
func GroupFilterEventTypes(types []datastore.ProjectEventType, observed []string) (catalogOut, observedOut []string) {
	catalogOut = CatalogFilterNames(types)
	declared := declaredFilterNames(types)
	inDeclared := make(map[string]struct{}, len(declared))
	for _, name := range declared {
		inDeclared[name] = struct{}{}
	}

	observedOut = make([]string, 0, len(observed))
	seen := make(map[string]struct{}, len(observed))
	for _, raw := range observed {
		name := strings.TrimSpace(raw)
		if name == "" || name == "*" {
			continue
		}
		if _, ok := inDeclared[name]; ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		observedOut = append(observedOut, name)
	}
	sort.Strings(observedOut)
	return catalogOut, observedOut
}

func eventMetadataTypeExistsSQL(arg string) string {
	return `EXISTS (SELECT 1 FROM convoy.events ev WHERE ev.id = ed.event_id AND ev.project_id = ed.project_id AND ev.event_type ` + arg + `)`
}

func observedEventTypesSQL(projectID string, start, end time.Time, endpointIDs []string) (string, []any) {
	var b strings.Builder
	args := make([]any, 0, 5)
	b.WriteString(`SELECT DISTINCT ev.event_type FROM convoy.event_deliveries ed INNER JOIN convoy.events ev ON ev.id = ed.event_id AND ev.project_id = ed.project_id WHERE ed.deleted_at IS NULL`)
	args = append(args, projectID)
	fmt.Fprintf(&b, ` AND ed.project_id = $%d`, len(args))
	args = append(args, start)
	fmt.Fprintf(&b, ` AND ed.created_at >= $%d`, len(args))
	args = append(args, end)
	fmt.Fprintf(&b, ` AND ed.created_at <= $%d`, len(args))
	b.WriteString(` AND ev.event_type <> '' AND ev.event_type <> '*'`)
	if len(endpointIDs) > 0 {
		args = append(args, endpointIDs)
		fmt.Fprintf(&b, ` AND ed.endpoint_id = ANY($%d::TEXT[])`, len(args))
	}
	fmt.Fprintf(&b, ` ORDER BY ev.event_type LIMIT %d`, FilterEventTypeLimit)
	return b.String(), args
}

// ObservedEventTypes returns distinct live event types in the date window,
// using the same COALESCE as the list table. Empty and "*" are excluded.
// When endpointIDs is set the predicate is a real ANY, not a CASE flag.
func (s *Service) ObservedEventTypes(ctx context.Context, projectID string, params datastore.SearchParams, endpointIDs []string) ([]string, error) {
	start, end := getCreatedDateFilter(params.CreatedAtStart, params.CreatedAtEnd)
	query, args := observedEventTypesSQL(projectID, start, end, endpointIDs)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name string
		if scanErr := rows.Scan(&name); scanErr != nil {
			return nil, scanErr
		}
		name = strings.TrimSpace(name)
		if name == "" || name == "*" {
			continue
		}
		names = append(names, name)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return names, nil
}
