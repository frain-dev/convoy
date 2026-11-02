-- +migrate Up
CREATE TABLE convoy.queue_execution_ownership (
 scope TEXT NOT NULL,
 task_name TEXT NOT NULL,
 task_id TEXT NOT NULL,
 store_id TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('running','retry','completed')),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(scope,task_name,task_id)
);
-- +migrate Down
DROP TABLE convoy.queue_execution_ownership;
