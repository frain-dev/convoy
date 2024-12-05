package utils

import (
	"fmt"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/spf13/cobra"
)

func AddPartitionCommand(a *cli.App) *cobra.Command {
	var table string

	cmd := &cobra.Command{
		Use:   "partition",
		Short: "runs partition commands",
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if table == "" {
				return fmt.Errorf("table name is required")
			}

			switch table {
			case "events":
				_, err := a.DB.GetDB().ExecContext(cmd.Context(), partitionEventsTable)
				if err != nil {
					return err
				}
			case "event-deliveries":
				_, err := a.DB.GetDB().ExecContext(cmd.Context(), partitionEventDeliveriesTable)
				if err != nil {
					return err
				}
			case "delivery-attempts":
				_, err := a.DB.GetDB().ExecContext(cmd.Context(), partitionDeliveryAttemptsTable)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown table %s", table)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&table, "table", "t", "", "table name")

	return cmd
}

func AddUnPartitionCommand(a *cli.App) *cobra.Command {
	var table string

	cmd := &cobra.Command{
		Use:   "unpartition",
		Short: "runs partition commands",
		Annotations: map[string]string{
			"CheckMigration":  "true",
			"ShouldBootstrap": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if table == "" {
				return fmt.Errorf("table name is required")
			}

			switch table {
			case "events":
				_, err := a.DB.GetDB().ExecContext(cmd.Context(), unPartitionEventsTable)
				if err != nil {
					return err
				}
			case "event-deliveries":
				_, err := a.DB.GetDB().ExecContext(cmd.Context(), unPartitionEventDeliveriesTable)
				if err != nil {
					return err
				}
			case "delivery-attempts":
				_, err := a.DB.GetDB().ExecContext(cmd.Context(), unPartitionDeliveryAttemptsTable)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown table %s", table)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&table, "table", "t", "", "table name")

	return cmd
}

var partitionEventsTable = `
CREATE OR REPLACE FUNCTION convoy.enforce_event_fk() 
    RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM convoy.events
        WHERE id = NEW.event_id
    ) THEN
        RAISE EXCEPTION 'Foreign key violation: event_id % does not exist in events', NEW.event_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION convoy.partition_events_table() 
    RETURNS VOID AS $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE 'Creating partitioned table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.events_new;

    -- Create partitioned table
    CREATE TABLE convoy.events_new (
        id                 VARCHAR NOT NULL,
        event_type         TEXT NOT NULL,
        endpoints          TEXT,
        project_id         VARCHAR NOT NULL REFERENCES convoy.projects,
        source_id          VARCHAR REFERENCES convoy.sources,
        headers            JSONB,
        raw                TEXT NOT NULL,
        data               BYTEA NOT NULL,
        created_at         TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        updated_at         TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
        deleted_at         TIMESTAMPTZ,
        url_query_params   VARCHAR,
        idempotency_key    TEXT,
        is_duplicate_event BOOLEAN DEFAULT FALSE,
        acknowledged_at    TIMESTAMPTZ,
        status             TEXT,
        metadata           TEXT,
        PRIMARY KEY (id, created_at, project_id)
    ) PARTITION BY RANGE (project_id, created_at);

    RAISE NOTICE 'Creating partitions...';
    FOR r IN
        WITH dates AS (
            SELECT project_id, created_at::DATE
            FROM convoy.events
            GROUP BY created_at::DATE, project_id
        )
        SELECT project_id,
               created_at::TEXT AS start_date,
               (created_at + 1)::TEXT AS stop_date,
               'events_' || pg_catalog.REPLACE(project_id::TEXT, '-', '') || '_' || pg_catalog.REPLACE(created_at::TEXT, '-', '') AS partition_table_name
        FROM dates
    LOOP

        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.events_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
            r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date
        );
    END LOOP;

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.events_new (
        id, event_type, endpoints, project_id, source_id, headers, raw, data,
        created_at, updated_at, deleted_at, url_query_params, idempotency_key,
        is_duplicate_event, acknowledged_at, status, metadata
    )
    SELECT id, event_type, endpoints, project_id, source_id, headers, raw, data,
           created_at, updated_at, deleted_at, url_query_params, idempotency_key,
           is_duplicate_event, acknowledged_at, status, metadata
    FROM convoy.events;

    -- Manage table renaming
    ALTER TABLE convoy.event_deliveries DROP CONSTRAINT IF EXISTS event_deliveries_event_id_fkey;
    ALTER TABLE convoy.events RENAME TO events_old;
    ALTER TABLE convoy.events_new RENAME TO events;
    DROP TABLE IF EXISTS convoy.events_old;

    RAISE NOTICE 'Recreating indexes...';
    CREATE INDEX idx_events_id_key ON convoy.events (id);
    CREATE INDEX idx_events_created_at_key ON convoy.events (created_at);
    CREATE INDEX idx_events_deleted_at_key ON convoy.events (deleted_at);
    CREATE INDEX idx_events_project_id_deleted_at_key ON convoy.events (project_id, deleted_at);
    CREATE INDEX idx_events_project_id_key ON convoy.events (project_id);
    CREATE INDEX idx_events_project_id_source_id ON convoy.events (project_id, source_id);
    CREATE INDEX idx_events_source_id ON convoy.events (source_id);
    CREATE INDEX idx_idempotency_key_key ON convoy.events (idempotency_key);
    CREATE INDEX idx_project_id_on_not_deleted ON convoy.events (project_id) WHERE deleted_at IS NULL;

    -- Recreate FK using trigger
    CREATE OR REPLACE TRIGGER event_fk_check
    BEFORE INSERT ON convoy.event_deliveries
    FOR EACH ROW EXECUTE FUNCTION convoy.enforce_event_fk();

    RAISE NOTICE 'Migration complete!';
END;
$$ LANGUAGE plpgsql;
select convoy.partition_events_table()
`

var unPartitionEventsTable = `
create or replace function convoy.un_partition_events_table() 
    returns VOID as $$
begin
	RAISE NOTICE 'Starting un-partitioning of events table...';
    
	-- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.events_new;

    -- Create partitioned table
    CREATE TABLE convoy.events_new
    (
        id                 VARCHAR not null primary key ,
        event_type         TEXT not null,
        endpoints          TEXT,
        project_id         VARCHAR not null
            constraint events_new_project_id_fkey
                references convoy.projects,
        source_id          VARCHAR
            constraint events_new_source_id_fkey
                references convoy.sources,
        headers            jsonb,
        raw                TEXT not null,
        data               bytea not null,
        created_at         TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP not null,
        updated_at         TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        deleted_at         TIMESTAMP WITH TIME ZONE,
        url_query_params   VARCHAR,
        idempotency_key    TEXT,
        is_duplicate_event BOOLEAN default false,
        acknowledged_at    TIMESTAMP WITH TIME ZONE,
        status             TEXT,
        metadata           TEXT
    );

    RAISE NOTICE 'Migrating data...';
    insert into convoy.events_new select * from convoy.events;
    ALTER TABLE convoy.event_deliveries DROP CONSTRAINT if exists event_deliveries_event_id_fkey;
    ALTER TABLE convoy.event_deliveries
        ADD CONSTRAINT event_deliveries_event_id_fkey
            FOREIGN KEY (event_id) REFERENCES convoy.events_new (id);

    ALTER TABLE convoy.events RENAME TO events_old;
    ALTER TABLE convoy.events_new RENAME TO events;
    DROP TABLE IF EXISTS convoy.events_old;

    RAISE NOTICE 'Recreating indexes...';
    CREATE INDEX idx_events_created_at_key ON convoy.events (created_at);
    CREATE INDEX idx_events_deleted_at_key ON convoy.events (deleted_at);
    CREATE INDEX idx_events_project_id_deleted_at_key ON convoy.events (project_id, deleted_at);
    CREATE INDEX idx_events_project_id_key ON convoy.events (project_id);
    CREATE INDEX idx_events_project_id_source_id ON convoy.events (project_id, source_id);
    CREATE INDEX idx_events_source_id ON convoy.events (source_id);
    CREATE INDEX idx_idempotency_key_key ON convoy.events (idempotency_key);
    CREATE INDEX idx_project_id_on_not_deleted ON convoy.events (project_id) WHERE deleted_at IS NULL;
	RAISE NOTICE 'Successfully un-partitioned events table...';
end $$ language plpgsql;
select convoy.un_partition_events_table()
`

var partitionEventDeliveriesTable = `
CREATE OR REPLACE FUNCTION enforce_event_delivery_fk()
    RETURNS TRIGGER AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM convoy.event_deliveries
        WHERE id = NEW.event_delivery_id
    ) THEN
        RAISE EXCEPTION 'Foreign key violation: event_delivery_id % does not exist in event deliveries', NEW.event_delivery_id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION partition_event_deliveries_table()
    RETURNS VOID AS $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE 'Creating partitioned event deliveries table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.event_deliveries_new;

    -- Create partitioned table
   create table convoy.event_deliveries_new
    (
        id               VARCHAR not null,
        status           TEXT    not null,
        description      TEXT    not null,
        project_id       VARCHAR not null references convoy.projects,
        endpoint_id      VARCHAR references convoy.endpoints,
        event_id         VARCHAR not null,
        device_id        VARCHAR references convoy.devices,
        subscription_id  VARCHAR not null references convoy.subscriptions,
        metadata         jsonb   not null,
        headers          jsonb,
        attempts         bytea,
        cli_metadata     jsonb,
        created_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        updated_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        deleted_at       TIMESTAMP WITH TIME ZONE,
        url_query_params VARCHAR,
        idempotency_key  TEXT,
        latency          TEXT,
        event_type       TEXT,
        acknowledged_at  TIMESTAMP WITH TIME ZONE,
        latency_seconds  NUMERIC,
        PRIMARY KEY (id, created_at, project_id)
    ) PARTITION BY RANGE (project_id, created_at);

    RAISE NOTICE 'Creating partitions...';
    FOR r IN
        WITH dates AS (
            SELECT project_id, created_at::DATE
            FROM convoy.event_deliveries
            GROUP BY created_at::DATE, project_id
            order by created_at::DATE
        )
        SELECT project_id,
               created_at::TEXT AS start_date,
               (created_at + 1)::TEXT AS stop_date,
               'event_deliveries_' || pg_catalog.REPLACE(project_id::TEXT, '-', '') || '_' || pg_catalog.REPLACE(created_at::TEXT, '-', '') AS partition_table_name
        FROM dates
    LOOP
        RAISE NOTICE '%', FORMAT ('CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.event_deliveries_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
            r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date);
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.event_deliveries_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
            r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date
        );
    END LOOP;

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.event_deliveries_new (
        id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
        attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
        latency_seconds
    )
    SELECT id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
           attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
           latency_seconds
    FROM convoy.event_deliveries;

    -- Manage table renaming
    ALTER TABLE convoy.delivery_attempts DROP CONSTRAINT IF EXISTS delivery_attempts_event_delivery_id_fkey;
    ALTER TABLE convoy.event_deliveries RENAME TO event_deliveries_old;
    ALTER TABLE convoy.event_deliveries_new RENAME TO event_deliveries;
    DROP TABLE IF EXISTS convoy.event_deliveries_old;

    RAISE NOTICE 'Recreating indexes...';
    create index event_deliveries_event_type on convoy.event_deliveries (event_type);
    create index idx_event_deliveries_created_at_key on convoy.event_deliveries (created_at);
    create index idx_event_deliveries_deleted_at_key on convoy.event_deliveries (deleted_at);
    create index idx_event_deliveries_device_id_key on convoy.event_deliveries (device_id);
    create index idx_event_deliveries_endpoint_id_key on convoy.event_deliveries (endpoint_id);
    create index idx_event_deliveries_event_id_key on convoy.event_deliveries (event_id);
    create index idx_event_deliveries_project_id_endpoint_id on convoy.event_deliveries (project_id, endpoint_id);
    create index idx_event_deliveries_project_id_endpoint_id_status on convoy.event_deliveries (project_id, endpoint_id, status);
    create index idx_event_deliveries_project_id_event_id on convoy.event_deliveries (project_id, event_id);
    create index idx_event_deliveries_project_id_key on convoy.event_deliveries (project_id);
    create index idx_event_deliveries_status on convoy.event_deliveries (status);
    create index idx_event_deliveries_status_key on convoy.event_deliveries (status);

    -- Recreate FK using trigger
    CREATE OR REPLACE TRIGGER event_delivery_fk_check
    BEFORE INSERT ON convoy.delivery_attempts
    FOR EACH ROW EXECUTE FUNCTION enforce_event_delivery_fk();

    RAISE NOTICE 'Migration complete!';
END;
$$ LANGUAGE plpgsql;
select partition_event_deliveries_table();
`

var unPartitionEventDeliveriesTable = `
create or replace function convoy.un_partition_event_deliveries_table() returns VOID as $$
begin
	RAISE NOTICE 'Starting un-partitioning of event deliveries table...';

	-- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.event_deliveries_new;

    -- Create partitioned table
    CREATE TABLE convoy.event_deliveries_new
    (
        id               VARCHAR not null primary key ,
        status           TEXT    not null,
        description      TEXT    not null,
        project_id       VARCHAR not null references convoy.projects,
        endpoint_id      VARCHAR references convoy.endpoints,
        event_id         VARCHAR not null,
        device_id        VARCHAR references convoy.devices,
        subscription_id  VARCHAR not null references convoy.subscriptions,
        metadata         jsonb   not null,
        headers          jsonb,
        attempts         bytea,
        cli_metadata     jsonb,
        created_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        updated_at       TIMESTAMP WITH TIME ZONE default CURRENT_TIMESTAMP,
        deleted_at       TIMESTAMP WITH TIME ZONE,
        url_query_params VARCHAR,
        idempotency_key  TEXT,
        latency          TEXT,
        event_type       TEXT,
        acknowledged_at  TIMESTAMP WITH TIME ZONE,
        latency_seconds  NUMERIC
    );

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.event_deliveries_new (
        id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
        attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
        latency_seconds
    )
    SELECT id, status, description, project_id, created_at, updated_at, endpoint_id, event_id, device_id, subscription_id, metadata, headers,
           attempts, cli_metadata, deleted_at, url_query_params, idempotency_key, latency, event_type, acknowledged_at,
           latency_seconds
    FROM convoy.event_deliveries;

    ALTER TABLE convoy.delivery_attempts DROP CONSTRAINT if exists delivery_attempts_event_delivery_id_fkey;
    ALTER TABLE convoy.delivery_attempts
        ADD CONSTRAINT delivery_attempts_event_delivery_id_fkey
            FOREIGN KEY (event_delivery_id) REFERENCES convoy.event_deliveries_new (id);

    ALTER TABLE convoy.event_deliveries RENAME TO event_deliveries_old;
    ALTER TABLE convoy.event_deliveries_new RENAME TO event_deliveries;
    DROP TABLE IF EXISTS convoy.event_deliveries_old;

    RAISE NOTICE 'Recreating indexes...';
    create index event_deliveries_event_type on convoy.event_deliveries (event_type);
    create index idx_event_deliveries_created_at_key on convoy.event_deliveries (created_at);
    create index idx_event_deliveries_deleted_at_key on convoy.event_deliveries (deleted_at);
    create index idx_event_deliveries_device_id_key on convoy.event_deliveries (device_id);
    create index idx_event_deliveries_endpoint_id_key on convoy.event_deliveries (endpoint_id);
    create index idx_event_deliveries_event_id_key on convoy.event_deliveries (event_id);
    create index idx_event_deliveries_project_id_endpoint_id on convoy.event_deliveries (project_id, endpoint_id);
    create index idx_event_deliveries_project_id_endpoint_id_status on convoy.event_deliveries (project_id, endpoint_id, status);
    create index idx_event_deliveries_project_id_event_id on convoy.event_deliveries (project_id, event_id);
    create index idx_event_deliveries_project_id_key on convoy.event_deliveries (project_id);
    create index idx_event_deliveries_status on convoy.event_deliveries (status);
    create index idx_event_deliveries_status_key on convoy.event_deliveries (status);

	RAISE NOTICE 'Successfully un-partitioned events table...';
end $$ language plpgsql;
select convoy.un_partition_event_deliveries_table()
`

var partitionDeliveryAttemptsTable = `
CREATE OR REPLACE FUNCTION partition_delivery_attempts_table()
    RETURNS VOID AS $$
DECLARE
    r RECORD;
BEGIN
    RAISE NOTICE 'Creating partitioned delivery attempts table...';

    -- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.delivery_attempts_new;

    -- Create partitioned table
   create table convoy.delivery_attempts_new
    (
        id                   VARCHAR not null,
        url                  TEXT    not null,
        method               VARCHAR not null,
        api_version          VARCHAR not null,
        project_id           VARCHAR not null references convoy.projects,
        endpoint_id          VARCHAR not null references convoy.endpoints,
        event_delivery_id    VARCHAR not null,
        ip_address           VARCHAR,
        request_http_header  jsonb,
        response_http_header jsonb,
        http_status          VARCHAR,
        response_data        bytea,
        error                TEXT,
        status               BOOLEAN,
        created_at           TIMESTAMP WITH TIME ZONE default now() not null,
        updated_at           TIMESTAMP WITH TIME ZONE default now() not null,
        deleted_at           TIMESTAMP WITH TIME ZONE,
        PRIMARY KEY (id, created_at, project_id)
    ) PARTITION BY RANGE (project_id, created_at);

    RAISE NOTICE 'Creating partitions...';
    FOR r IN
        WITH dates AS (
            SELECT project_id, created_at::DATE
            FROM convoy.delivery_attempts
            GROUP BY created_at::DATE, project_id
            order by created_at::DATE
        )
        SELECT project_id,
               created_at::TEXT AS start_date,
               (created_at + 1)::TEXT AS stop_date,
               'delivery_attempts_' || pg_catalog.REPLACE(project_id::TEXT, '-', '') || '_' || pg_catalog.REPLACE(created_at::TEXT, '-', '') AS partition_table_name
        FROM dates
    LOOP
        EXECUTE FORMAT(
            'CREATE TABLE IF NOT EXISTS convoy.%s PARTITION OF convoy.delivery_attempts_new FOR VALUES FROM (%L, %L) TO (%L, %L)',
            r.partition_table_name, r.project_id, r.start_date, r.project_id, r.stop_date
        );
    END LOOP;

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.delivery_attempts_new (
        id, url, method, api_version, project_id, endpoint_id,
        event_delivery_id, ip_address, request_http_header, response_http_header,
        http_status, response_data, error, status, created_at,
        updated_at, deleted_at
    )
    SELECT id, url, method, api_version, project_id, endpoint_id,
        event_delivery_id, ip_address, request_http_header, response_http_header,
        http_status, response_data, error, status, created_at,
        updated_at, deleted_at
    FROM convoy.delivery_attempts;

    -- Manage table renaming
    ALTER TABLE convoy.delivery_attempts RENAME TO delivery_attempts_old;
    ALTER TABLE convoy.delivery_attempts_new RENAME TO delivery_attempts;
    DROP TABLE IF EXISTS convoy.delivery_attempts_old;

    RAISE NOTICE 'Recreating indexes...';
    create index idx_delivery_attempts_created_at on convoy.delivery_attempts (created_at);
    create index idx_delivery_attempts_created_at_id_event_delivery_id
        on convoy.delivery_attempts using brin (created_at, id, project_id, event_delivery_id)
        where (deleted_at IS NULL);
    create index idx_delivery_attempts_event_delivery_id
        on convoy.delivery_attempts (event_delivery_id);
    create index idx_delivery_attempts_event_delivery_id_created_at
        on convoy.delivery_attempts (event_delivery_id, created_at);
    create index idx_delivery_attempts_event_delivery_id_created_at_desc
        on convoy.delivery_attempts (event_delivery_id asc, created_at desc);

    RAISE NOTICE 'Migration complete!';
END;
$$ LANGUAGE plpgsql;
select partition_delivery_attempts_table();
`

var unPartitionDeliveryAttemptsTable = `
create or replace function convoy.un_partition_delivery_attempts_table() returns VOID as $$
begin
	RAISE NOTICE 'Starting un-partitioning of delivery attempts table...';

	-- Drop old partitioned table
    DROP TABLE IF EXISTS convoy.delivery_attempts_new;

    -- Create partitioned table
    create table convoy.delivery_attempts_new
    (
        id                   VARCHAR not null primary key,
        url                  TEXT    not null,
        method               VARCHAR not null,
        api_version          VARCHAR not null,
        project_id           VARCHAR not null references convoy.projects,
        endpoint_id          VARCHAR not null references convoy.endpoints,
        event_delivery_id    VARCHAR not null,
        ip_address           VARCHAR,
        request_http_header  jsonb,
        response_http_header jsonb,
        http_status          VARCHAR,
        response_data        bytea,
        error                TEXT,
        status               BOOLEAN,
        created_at           TIMESTAMP WITH TIME ZONE default now() not null,
        updated_at           TIMESTAMP WITH TIME ZONE default now() not null,
        deleted_at           TIMESTAMP WITH TIME ZONE
    );

    RAISE NOTICE 'Migrating data...';
    INSERT INTO convoy.delivery_attempts_new (
        id, url, method, api_version, project_id, endpoint_id,
        event_delivery_id, ip_address, request_http_header, response_http_header,
        http_status, response_data, error, status, created_at,
        updated_at, deleted_at
    )
    SELECT id, url, method, api_version, project_id, endpoint_id,
           event_delivery_id, ip_address, request_http_header, response_http_header,
           http_status, response_data, error, status, created_at,
           updated_at, deleted_at
    FROM convoy.delivery_attempts;

    ALTER TABLE convoy.delivery_attempts RENAME TO delivery_attempts_old;
    ALTER TABLE convoy.delivery_attempts_new RENAME TO delivery_attempts;
    DROP TABLE IF EXISTS convoy.delivery_attempts_old;

    RAISE NOTICE 'Recreating indexes...';
	create index idx_delivery_attempts_created_at on convoy.delivery_attempts (created_at);
    create index idx_delivery_attempts_created_at_id_event_delivery_id
        on convoy.delivery_attempts using brin (created_at, id, project_id, event_delivery_id)
        where (deleted_at IS NULL);
    create index idx_delivery_attempts_event_delivery_id
        on convoy.delivery_attempts (event_delivery_id);
    create index idx_delivery_attempts_event_delivery_id_created_at
        on convoy.delivery_attempts (event_delivery_id, created_at);
    create index idx_delivery_attempts_event_delivery_id_created_at_desc
        on convoy.delivery_attempts (event_delivery_id asc, created_at desc);

	RAISE NOTICE 'Successfully un-partitioned delivery attempts table...';
end $$ language plpgsql;
select convoy.un_partition_delivery_attempts_table()
`
