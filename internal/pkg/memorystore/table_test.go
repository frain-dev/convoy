package memorystore

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Table_Add(t *testing.T) {
	tests := map[string]struct {
		setupTable func(table *Table)
	}{
		"should_return_row_if_key_exists": {
			setupTable: func(table *Table) {
				table.Add("test-key", "test-value")
			},
		},
		"should_return_row_if_does_not_exist": {},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			table := NewTable()

			if tc.setupTable != nil {
				tc.setupTable(table)
			}

			row := table.Add("test-key", "test-value")

			require.Equal(t, len(table.GetKeys()), 1)
			require.Equal(t, row.Key(), "test-key")
			require.Equal(t, row.Value(), "test-value")
		})
	}
}

func Test_Table_GetKeys(t *testing.T) {
	table := NewTable()
	numberOfItems := rand.Intn(10)

	for i := 0; i < numberOfItems; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		table.Add(key, "test-value")
	}

	require.Equal(t, len(table.GetKeys()), numberOfItems)
}
