package events

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/events/repo"
)

// rowToEvent converts various sqlc-generated row types to datastore.Event
// Handles: FindEventByIDRow, LoadEventsPagedExistsRow, LoadEventsPagedSearchRow, etc.
func rowToEvent(row interface{}) (*datastore.Event, error) {
	// TODO: Implement type switching for different row types
	// Example pattern:
	// switch r := row.(type) {
	// case repo.FindEventByIDRow:
	//     return &datastore.Event{
	//         UID:       r.ID,
	//         EventType: datastore.EventType(r.EventType),
	//         ...
	//     }, nil
	// case repo.LoadEventsPagedExistsRow:
	//     ...
	// }
	panic("not implemented")
}

// rowsToEvents converts multiple rows to events
func rowsToEvents(rows interface{}) ([]datastore.Event, error) {
	// TODO: Implement batch conversion
	panic("not implemented")
}

// parseEndpoints converts JSONB array to []string
func parseEndpoints(endpointsJSON interface{}) ([]string, error) {
	// TODO: Parse endpoints from JSONB or TEXT array
	panic("not implemented")
}

// endpointsToString converts []string to TEXT for storage
func endpointsToString(endpoints []string) string {
	// TODO: Convert to comma-separated or appropriate format
	panic("not implemented")
}

// convertTimestamp converts pgtype.Timestamptz to time.Time
// func convertTimestamp(ts pgtype.Timestamptz) time.Time {
// 	// TODO: Handle nullable timestamps
// 	panic("not implemented")
// }

// convertNullableString converts pgtype.Text to string
// func convertNullableString(s pgtype.Text) string {
// 	// TODO: Handle NULL values
// 	panic("not implemented")
// }

// buildEventEndpoints creates event_endpoints insert params for batch processing
func buildEventEndpoints(eventID string, endpoints []string) []repo.CreateEventEndpointsParams {
	// TODO: Create params slice for batch insert
	// Partition into 30K chunks if needed
	panic("not implemented")
}
