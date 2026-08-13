package testenv

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/database"
)

// RequirePartitionsAddressableByRetention asserts every child of convoy.<parent>
// is named <parent>_<TENANT>_<YYYYMMDD>, with TENANT byte-identical to
// tenantID.
//
// Retention adopts a partition only if it can parse the name back into a tenant
// and a single day, and it builds the partitions it creates for later days with
// an upper-case tenant segment. Postgres folds unquoted identifiers to lower
// case, so partition DDL has to both upper-case the tenant id and quote the
// resulting identifier, or one table ends up spelled two ways.
//
// gopartman v0.2.0 adopts a folded name too, which is what lets an instance
// partitioned before that fix heal on upgrade rather than needing every child
// renamed. This asserts the name we mean to write, not the widest name adoption
// happens to accept.
//
// This reads pg_class rather than the generated SQL because the folding happens
// inside Postgres: the statement can look correct while the stored name is not.
// It lives here so every partitioned table asserts the same contract.
func RequirePartitionsAddressableByRetention(t *testing.T, db database.Database, parent, tenantID string) {
	t.Helper()

	rows, err := db.GetDB().QueryContext(context.Background(), `
        SELECT c.relname
        FROM pg_class c
        JOIN pg_inherits i ON i.inhrelid = c.oid
        JOIN pg_class p ON p.oid = i.inhparent
        JOIN pg_namespace n ON n.oid = p.relnamespace
        WHERE n.nspname = 'convoy' AND p.relname = $1`, parent)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	wantPrefix := fmt.Sprintf("%s_%s_", parent, strings.ToUpper(strings.ReplaceAll(tenantID, "-", "")))

	var found int
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		found++
		require.True(t, strings.HasPrefix(name, wantPrefix),
			"partition %q is not addressable by retention, want prefix %q", name, wantPrefix)
	}
	require.NoError(t, rows.Err())

	// A partition helper only creates children for days that already hold rows,
	// so an empty parent would vacuously pass every assertion above.
	require.NotZero(t, found, "convoy.%s has no partitions", parent)
}
