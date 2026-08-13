package delivery_attempts

import (
	"context"

	"github.com/frain-dev/convoy/internal/pkg/attach"
)

func (s *Service) PartitionDeliveryAttemptsTable(ctx context.Context) error {
	return attach.Convert(ctx, s.db, attemptsSpec)
}

func (s *Service) UnPartitionDeliveryAttemptsTable(ctx context.Context) error {
	return attach.Revert(ctx, s.db, attemptsSpec)
}

var attemptsSpec = attach.Spec{
	Table: "delivery_attempts",
	ParentForeignKeys: `FOREIGN KEY (project_id) REFERENCES convoy.projects,
    FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints`,
	SwapEnd:         []string{attach.AttemptFKSQL},
	AfterDetach:     []string{attach.RestoreAttemptFKSQL},
	CopyUnpartition: unPartitionDeliveryAttemptsTableSQL,
}
