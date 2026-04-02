package backup_collector

import (
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pglogrepl"
	"github.com/stretchr/testify/require"
)

func TestBuffer_Append(t *testing.T) {
	buf := NewBuffer()

	buf.Append("events", map[string]string{"id": "1", "project_id": "p1"}, 100)
	buf.Append("events", map[string]string{"id": "2", "project_id": "p1"}, 200)
	buf.Append("event_deliveries", map[string]string{"id": "3", "project_id": "p1"}, 300)

	require.Equal(t, 3, buf.Len())

	records, lsn := buf.Swap()
	require.Len(t, records["events"], 2)
	require.Len(t, records["event_deliveries"], 1)
	require.Equal(t, pglogrepl.LSN(300), lsn)
}

func TestBuffer_Append_Concurrent(t *testing.T) {
	buf := NewBuffer()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			buf.Append("events", map[string]string{
				"id":         fmt.Sprintf("e%d", n),
				"project_id": "p1",
			}, pglogrepl.LSN(n))
		}(i)
	}

	wg.Wait()
	require.Equal(t, 100, buf.Len())
}

func TestBuffer_Swap(t *testing.T) {
	buf := NewBuffer()

	buf.Append("events", map[string]string{"id": "1"}, 100)
	buf.Append("events", map[string]string{"id": "2"}, 200)

	records, lsn := buf.Swap()
	require.Len(t, records["events"], 2)
	require.Equal(t, pglogrepl.LSN(200), lsn)

	// After swap, buffer is empty
	require.Equal(t, 0, buf.Len())

	// Second swap returns nothing
	records2, lsn2 := buf.Swap()
	require.Len(t, records2, 0)
	require.Equal(t, pglogrepl.LSN(0), lsn2)
}

func TestBuffer_Swap_Empty(t *testing.T) {
	buf := NewBuffer()

	records, lsn := buf.Swap()
	require.Len(t, records, 0)
	require.Equal(t, pglogrepl.LSN(0), lsn)
}

func TestBuffer_LSN_Tracking(t *testing.T) {
	buf := NewBuffer()

	// LSN should track the maximum
	buf.Append("events", map[string]string{"id": "1"}, 500)
	buf.Append("events", map[string]string{"id": "2"}, 100) // lower LSN
	buf.Append("events", map[string]string{"id": "3"}, 999) // highest

	_, lsn := buf.Swap()
	require.Equal(t, pglogrepl.LSN(999), lsn)
}
