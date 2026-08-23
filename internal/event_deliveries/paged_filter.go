package event_deliveries

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
)

// listFilter is the event-deliveries list/count scan. Optional clauses are
// omitted entirely so the planner sees real predicates (status = ANY, keyset
// tuple compare) instead of CASE WHEN @flag THEN ... ELSE true END.
type listFilter struct {
	ProjectID       string
	EventID         string
	EventType       string
	Start           time.Time
	End             time.Time
	EndpointIDs     []string
	Statuses        []string
	SubscriptionID  string
	BrokerMessageID string
	IdempotencyKey  string

	DeliveryID      string
	EventIDSearch   string
	TypePrefix      string
	SearchEndpoints []string

	HasKeyset bool
	KeysetAt  time.Time
	KeysetID  string
	KeysetOp  string // "<=" or ">=" for the page scan; ">" or "<" for prev-count
}

func (f listFilter) appendWhere(b *strings.Builder, args []any) []any {
	b.WriteString(`FROM convoy.event_deliveries ed WHERE ed.deleted_at IS NULL`)
	args = append(args, f.ProjectID)
	fmt.Fprintf(b, ` AND ed.project_id = $%d`, len(args))
	args = append(args, f.Start)
	fmt.Fprintf(b, ` AND ed.created_at >= $%d`, len(args))
	args = append(args, f.End)
	fmt.Fprintf(b, ` AND ed.created_at <= $%d`, len(args))

	if f.EventID != "" {
		args = append(args, f.EventID)
		fmt.Fprintf(b, ` AND ed.event_id = $%d`, len(args))
	}
	if f.EventType != "" {
		args = append(args, f.EventType)
		fmt.Fprintf(b, ` AND ed.event_type = $%d`, len(args))
	}
	if len(f.EndpointIDs) > 0 {
		args = append(args, f.EndpointIDs)
		fmt.Fprintf(b, ` AND ed.endpoint_id = ANY($%d::TEXT[])`, len(args))
	}
	if len(f.Statuses) > 0 {
		args = append(args, f.Statuses)
		fmt.Fprintf(b, ` AND ed.status = ANY($%d::TEXT[])`, len(args))
	}
	if f.SubscriptionID != "" {
		args = append(args, f.SubscriptionID)
		fmt.Fprintf(b, ` AND ed.subscription_id = $%d`, len(args))
	}
	if f.BrokerMessageID != "" {
		args = append(args, f.BrokerMessageID)
		fmt.Fprintf(b, ` AND ed.headers -> 'x-broker-message-id' ->> 0 = $%d`, len(args))
	}
	if f.IdempotencyKey != "" {
		args = append(args, f.IdempotencyKey)
		fmt.Fprintf(b, ` AND ed.idempotency_key = $%d`, len(args))
	}

	switch {
	case f.DeliveryID != "":
		args = append(args, f.DeliveryID)
		fmt.Fprintf(b, ` AND ed.id = $%d`, len(args))
	case f.EventIDSearch != "":
		args = append(args, f.EventIDSearch)
		fmt.Fprintf(b, ` AND ed.event_id = $%d`, len(args))
	case f.TypePrefix != "" || len(f.SearchEndpoints) > 0:
		var parts []string
		if f.TypePrefix != "" {
			args = append(args, f.TypePrefix)
			parts = append(parts, fmt.Sprintf(`ed.event_type ILIKE $%d ESCAPE '\'`, len(args)))
		}
		if len(f.SearchEndpoints) > 0 {
			args = append(args, f.SearchEndpoints)
			parts = append(parts, fmt.Sprintf(`ed.endpoint_id = ANY($%d::TEXT[])`, len(args)))
		}
		b.WriteString(` AND (`)
		b.WriteString(strings.Join(parts, ` OR `))
		b.WriteString(`)`)
	}

	if f.HasKeyset {
		args = append(args, f.KeysetAt)
		at := len(args)
		args = append(args, f.KeysetID)
		fmt.Fprintf(b, ` AND (ed.created_at, ed.id) %s ($%d, $%d)`, f.KeysetOp, at, len(args))
	}
	return args
}

func (f listFilter) pageSQL(limit int, desc bool) (string, []any) {
	var b strings.Builder
	b.WriteString(`SELECT ed.id `)
	args := f.appendWhere(&b, nil)
	if desc {
		b.WriteString(` ORDER BY ed.created_at DESC, ed.id DESC`)
	} else {
		b.WriteString(` ORDER BY ed.created_at ASC, ed.id ASC`)
	}
	args = append(args, limit)
	fmt.Fprintf(&b, ` LIMIT $%d`, len(args))
	return b.String(), args
}

func (f listFilter) countSQL() (string, []any) {
	var b strings.Builder
	b.WriteString(`SELECT COUNT(*) `)
	args := f.appendWhere(&b, nil)
	return b.String(), args
}

func escapeLike(q string) string {
	q = strings.ReplaceAll(q, `\`, `\\`)
	q = strings.ReplaceAll(q, `%`, `\%`)
	q = strings.ReplaceAll(q, `_`, `\_`)
	return q
}

func looksLikeID(q string) bool {
	if q == "" || strings.ContainsRune(q, '.') {
		return false
	}
	n := 0
	for _, r := range q {
		if unicode.IsSpace(r) {
			return false
		}
		n++
	}
	return n >= 20
}

func (s *Service) resolveCursor(ctx context.Context, projectID, cursor string) (time.Time, string, bool, error) {
	if strings.TrimSpace(cursor) == "" {
		return time.Time{}, "", false, nil
	}
	var at time.Time
	var id string
	err := s.db.QueryRow(ctx,
		`SELECT created_at, id FROM convoy.event_deliveries WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`,
		cursor, projectID,
	).Scan(&at, &id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, "", false, nil
		}
		return time.Time{}, "", false, err
	}
	return at, id, true, nil
}

func (s *Service) applySearch(ctx context.Context, f *listFilter, q string) error {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}

	if looksLikeID(q) {
		var exists bool
		err := s.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM convoy.event_deliveries WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL)`,
			q, f.ProjectID,
		).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			f.DeliveryID = q
			return nil
		}

		err = s.db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM convoy.event_deliveries WHERE event_id = $1 AND project_id = $2 AND deleted_at IS NULL)`,
			q, f.ProjectID,
		).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			f.EventIDSearch = q
			return nil
		}
	}

	f.TypePrefix = escapeLike(q) + "%"

	rows, err := s.db.Query(ctx,
		`SELECT id FROM convoy.endpoints WHERE project_id = $1 AND deleted_at IS NULL AND name ILIKE $2 ESCAPE '\' LIMIT 200`,
		f.ProjectID, "%"+escapeLike(q)+"%",
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		f.SearchEndpoints = append(f.SearchEndpoints, id)
	}
	return rows.Err()
}

func (s *Service) queryDeliveryIDs(ctx context.Context, f listFilter, limit int, desc bool) ([]string, error) {
	sql, args := f.pageSQL(limit, desc)
	rows, err := s.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Service) queryDeliveryCount(ctx context.Context, f listFilter) (int64, error) {
	sql, args := f.countSQL()
	var n int64
	if err := s.db.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
