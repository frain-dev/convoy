package event_deliveries

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
)

// CountRetryCandidates sums matching deliveries per project. An empty
// projectIDs slice is zero, not an instance-wide wildcard: the empty-string
// COUNT plan can undercount on a partitioned table while the retry walk
// pages each live project.
func (s *Service) CountRetryCandidates(ctx context.Context, projectIDs []string, statuses []datastore.EventDeliveryStatus, eventID string, params datastore.SearchParams) (int64, error) {
	if len(projectIDs) == 0 || len(statuses) == 0 {
		return 0, nil
	}

	var total int64
	for _, projectID := range projectIDs {
		if eventID != "" {
			n, err := s.CountEventDeliveries(ctx, projectID, nil, eventID, statuses, params)
			if err != nil {
				return 0, err
			}
			total += n
			continue
		}
		for _, status := range statuses {
			n, err := s.CountDeliveriesByStatus(ctx, projectID, status, params)
			if err != nil {
				return 0, err
			}
			total += n
		}
	}
	return total, nil
}
