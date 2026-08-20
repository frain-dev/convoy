package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

const SearchTimeout = 5 * time.Second

var (
	ErrSearchUnlicensed   = errors.New("event search requires the Event Search entitlement")
	ErrInvalidPayloadBody = errors.New("body must be a nonempty JSON object")
)

// ListSearchSQL carries the bound parameters for unified list search on convoy.events.
type ListSearchSQL struct {
	HasSearch      bool
	HasQuery       bool
	SearchIDPrefix string
	SearchContains string
	HasBody        bool
	Body           []byte
}

// ApplyEventListSearch normalises GET /events search parameters.
//
// Licensed users search list metadata and payload within the active date window.
// Text in query matches metadata. A strict JSON object in query is promoted to
// body containment. Mixed leftover text plus a JSON object (or query= with body=)
// AND together. Unlicensed instances reject all search.
func ApplyEventListSearch(filter *datastore.Filter, project *datastore.Project, licensed bool, now time.Time) error {
	if filter == nil {
		return nil
	}

	filter.EventSearchLicensed = licensed

	hasQuery := !util.IsStringEmpty(filter.Query)
	hasBody := len(filter.Body) > 0
	if !hasQuery && !hasBody {
		return nil
	}
	if !licensed {
		return ErrSearchUnlicensed
	}

	if hasQuery && !hasBody {
		if body, ok := queryAsPayloadBody(filter.Query); ok {
			filter.Body = body
			filter.Query = ""
			hasQuery = false
			hasBody = true
		} else if rest, body, ok := splitQueryAndPayload(filter.Query); ok {
			filter.Query = rest
			filter.Body = body
			hasQuery = true
			hasBody = true
		}
	}

	if hasBody {
		if err := validatePayloadBody(filter.Body); err != nil {
			return err
		}
	}

	return nil
}

func ListSearchSQLFromFilter(filter *datastore.Filter, project *datastore.Project) ListSearchSQL {
	if filter == nil {
		return ListSearchSQL{}
	}

	hasQuery := !util.IsStringEmpty(filter.Query)
	hasBody := len(filter.Body) > 0
	if !hasQuery && !hasBody {
		return ListSearchSQL{}
	}

	out := ListSearchSQL{
		HasSearch: true,
		HasBody:   hasBody,
		Body:      bytes.TrimSpace(filter.Body),
	}
	if hasQuery {
		trimmed := strings.TrimSpace(filter.Query)
		out.HasQuery = true
		out.SearchIDPrefix = escapeLikePattern(trimmed) + "%"
		out.SearchContains = "%" + escapeLikePattern(trimmed) + "%"
	}
	return out
}

func NeedsSearchTimeout(filter *datastore.Filter, project *datastore.Project) bool {
	sql := ListSearchSQLFromFilter(filter, project)
	return sql.HasBody
}

func splitQueryAndPayload(query string) (rest string, body json.RawMessage, ok bool) {
	trimmed := strings.TrimSpace(query)
	start := strings.IndexByte(trimmed, '{')
	end := strings.LastIndexByte(trimmed, '}')
	if start < 0 || end <= start {
		return "", nil, false
	}
	body, ok = queryAsPayloadBody(trimmed[start : end+1])
	if !ok {
		return "", nil, false
	}
	rest = strings.TrimSpace(trimmed[:start] + trimmed[end+1:])
	if rest == "" {
		return "", nil, false
	}
	return rest, body, true
}

func queryAsPayloadBody(query string) (json.RawMessage, bool) {
	trimmed := bytes.TrimSpace([]byte(query))
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, false
	}
	obj, ok := value.(map[string]any)
	if !ok || len(obj) == 0 {
		return nil, false
	}
	return json.RawMessage(trimmed), true
}

func validatePayloadBody(body json.RawMessage) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return ErrInvalidPayloadBody
	}
	var value any
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return ErrInvalidPayloadBody
	}
	obj, ok := value.(map[string]any)
	if !ok || len(obj) == 0 {
		return ErrInvalidPayloadBody
	}
	return nil
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

// IsSearchTimeout reports query timeouts for payload-heavy list search.
func IsSearchTimeout(err error) bool {
	return IsPayloadSearchTimeout(err)
}

func IsPayloadSearchTimeout(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "57014"
}
