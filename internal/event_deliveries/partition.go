package event_deliveries

import (
	"context"

	"github.com/frain-dev/convoy/internal/pkg/attach"
)

func (s *Service) PartitionEventDeliveriesTable(ctx context.Context) error {
	return attach.Convert(ctx, s.db, deliveriesSpec)
}

func (s *Service) UnPartitionEventDeliveriesTable(ctx context.Context) error {
	return attach.Revert(ctx, s.db, deliveriesSpec)
}

var deliveriesSpec = attach.Spec{
	Table: "event_deliveries",
	ParentForeignKeys: `FOREIGN KEY (project_id) REFERENCES convoy.projects,
    FOREIGN KEY (endpoint_id) REFERENCES convoy.endpoints,
    FOREIGN KEY (device_id) REFERENCES convoy.devices,
    FOREIGN KEY (subscription_id) REFERENCES convoy.subscriptions`,
	ExtraNotNull: []string{"delivery_mode"},
	ValidateHint: "If this failed on a null, backfill with " +
		"UPDATE convoy.event_deliveries SET created_at = COALESCE(created_at, NOW()), " +
		"delivery_mode = COALESCE(delivery_mode, 'at_least_once') " +
		"WHERE created_at IS NULL OR delivery_mode IS NULL, then run the conversion again",
	Swap: append(
		attach.DropConstraintSQL("delivery_attempts", "delivery_attempts_event_delivery_id_fkey"),
		attach.AttemptFKSQL,
	),
	AfterAttach:     []string{attach.EventFKSQL},
	AfterDetach:     []string{attach.RestoreAttemptFKSQL, attach.RestoreEventFKSQL},
	CopyUnpartition: unPartitionEventDeliveriesTableSQL,
}
