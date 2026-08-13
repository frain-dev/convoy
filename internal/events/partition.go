package events

import (
	"context"

	"github.com/frain-dev/convoy/internal/pkg/attach"
)

func (s *Service) PartitionEventsTable(ctx context.Context) error {
	return attach.Convert(ctx, s.db, eventsSpec)
}

func (s *Service) UnPartitionEventsTable(ctx context.Context) error {
	return attach.Revert(ctx, s.db, eventsSpec)
}

func (s *Service) PartitionEventsSearchTable(ctx context.Context) error {
	return attach.Convert(ctx, s.db, eventsSearchSpec)
}

func (s *Service) UnPartitionEventsSearchTable(ctx context.Context) error {
	return attach.Revert(ctx, s.db, eventsSearchSpec)
}

var eventsSpec = attach.Spec{
	Table: "events",
	ParentForeignKeys: `FOREIGN KEY (project_id) REFERENCES convoy.projects,
    FOREIGN KEY (source_id) REFERENCES convoy.sources`,
	Prepare: []string{
		`ALTER TABLE convoy.events ADD COLUMN IF NOT EXISTS url_path VARCHAR NOT NULL DEFAULT ''`,
	},
	Swap: []string{
		`ALTER TABLE convoy.event_deliveries DROP CONSTRAINT IF EXISTS event_deliveries_event_id_fkey`,
		attach.EventFKSQL,
	},
	AfterDetach:     []string{attach.RestoreEventFKSQL},
	CopyUnpartition: unPartitionEventsTableSQL,
}

var eventsSearchSpec = attach.Spec{
	Table: "events_search",
	ParentForeignKeys: `FOREIGN KEY (project_id) REFERENCES convoy.projects,
    FOREIGN KEY (source_id) REFERENCES convoy.sources`,
	Prepare: []string{
		`ALTER TABLE convoy.events_search ADD COLUMN IF NOT EXISTS url_path VARCHAR NOT NULL DEFAULT ''`,
	},
	CopyUnpartition: unPartitionEventsSearchTableSQL,
}
