package indexes

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Adopt drops every invalid heap index the catalog still holds that nothing has
// taken responsibility for yet, recording each in dropped_indexes the way a
// migration would. Boot runs this before the rebuild walker so indexes that
// turned invalid after the last migrate are not invisible until the next upgrade.
//
// Indexes reported as busy are left alone: a build in progress is also marked
// invalid until it finishes, and dropping it would destroy work someone is
// waiting on.
func Adopt(ctx context.Context, db *pgxpool.Pool) (int, error) {
	invalid, err := ListInvalid(ctx, db)
	if err != nil {
		return 0, err
	}

	var adopted int
	for _, i := range invalid {
		if i.Busy {
			continue
		}
		var dropped bool
		if err := db.QueryRow(ctx, `SELECT convoy.drop_invalid_index($1)`, i.Name).Scan(&dropped); err != nil {
			return adopted, fmt.Errorf("adopting invalid index %s: %w", i.Name, err)
		}
		if dropped {
			adopted++
		}
	}
	return adopted, nil
}
