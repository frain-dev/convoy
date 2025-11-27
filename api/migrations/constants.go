package migrations

// UpdateEndpointOwnerIDHeader is a header that migrations can set to signal that endpoint owner_ids need to be updated
// The business logic will handle the actual updates within the existing transaction
const UpdateEndpointOwnerIDHeader = "X-Migration-Update-Endpoint-Owner-ID"

