package configuration

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/configuration/repo"
)

func TestRowToConfiguration_AdminManagedState(t *testing.T) {
	tests := []struct {
		name    string
		value   pgtype.Bool
		managed bool
		known   bool
	}{
		{
			name:  "legacy row",
			value: pgtype.Bool{},
		},
		{
			name:  "environment managed",
			value: pgtype.Bool{Bool: false, Valid: true},
			known: true,
		},
		{
			name:    "admin managed",
			value:   pgtype.Bool{Bool: true, Valid: true},
			managed: true,
			known:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := rowToConfiguration(repo.LoadConfigurationRow{
				AdminManaged: tt.value,
			})

			require.Equal(t, tt.managed, cfg.AdminManaged)
			require.Equal(t, tt.known, cfg.AdminManagedKnown)
		})
	}
}
