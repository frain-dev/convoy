package memorystore

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

var prefix = "project-id-123"

func Test_Table_Add(t *testing.T) {

	t.Run("should return row if key exists", func(t *testing.T) {
		table := NewTable()

		key := NewKey(prefix, "test-key")
		value := "test-value"

		table.Add(key, value)

		// get row from table.
		row := table.Get(key)

		require.Equal(t, len(table.GetKeys()), 1)
		require.Equal(t, row.Value(), value)
	})
}

func Test_Table_GetKeys(t *testing.T) {
	table := NewTable()
	numberOfItems := rand.Intn(10)

	for i := 0; i < numberOfItems; i++ {
		key := NewKey(prefix, fmt.Sprintf("test-key-%d", i))
		table.Add(key, "test-value")
	}

	require.Equal(t, len(table.GetKeys()), numberOfItems)
}
